package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ravenmk2/rune-market/internal/store"
)

const (
	// SessionCookieName carries the opaque session token (HttpOnly, SameSite=Lax).
	SessionCookieName = "rm_session"

	sessionTTL      = 14 * 24 * time.Hour // design §7
	renewBelow      = 7 * 24 * time.Hour  // sliding renewal threshold
	settingRegMode  = "registration_mode"
	regModeOpen     = "open"
	regModeApproval = "approval"
	regModeClosed   = "closed"
)

const (
	ctxUserKey    = "auth.user"
	ctxSessionKey = "auth.session"
)

// ErrFounderProtected guards the founder account from delete/disable/demote (§7).
var ErrFounderProtected = errors.New("auth: founder account cannot be deleted, disabled or demoted")

// EnsureMutable enforces founder protection on a mutation target.
func EnsureMutable(u *store.User) error {
	if u.IsFounder {
		return ErrFounderProtected
	}
	return nil
}

type Service struct {
	users    UserRepository
	sessions SessionRepository
	settings SettingRepository
	limiter  *ipLimiter
}

func NewService(users UserRepository, sessions SessionRepository, settings SettingRepository) *Service {
	return &Service{
		users:    users,
		sessions: sessions,
		settings: settings,
		limiter:  newIPLimiter(10, time.Minute),
	}
}

func (s *Service) RegisterRoutes(g *gin.RouterGroup) {
	g.POST("/register", s.register)
	g.POST("/login", s.login)
	g.POST("/logout", s.logout)
	g.GET("/me", s.RequireAuth(), s.me)
}

// dummyHash keeps login timing uniform when the username does not exist.
var dummyHash, _ = HashPassword("dummy-password-for-timing")

type credentialsRequest struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

func (s *Service) register(c *gin.Context) {
	if !s.limiter.allow(c.ClientIP()) {
		Error(c, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
		return
	}
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if details := ValidateCredentials(req.Username, req.Nickname, req.Password); len(details) > 0 {
		ErrorDetails(c, http.StatusBadRequest, "invalid_argument", "validation failed", details)
		return
	}

	mode := s.registrationMode(c)
	if mode == regModeClosed {
		Error(c, http.StatusForbidden, "registration_closed", "registration is closed")
		return
	}
	status := store.StatusActive
	if mode == regModeApproval {
		status = store.StatusPending
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		Error(c, http.StatusInternalServerError, "internal", "failed to hash password")
		return
	}
	now := store.Now()
	u := &store.User{
		ID:           store.NewID(),
		Username:     req.Username,
		Nickname:     req.Nickname,
		PasswordHash: hash,
		Role:         store.RoleUser,
		Status:       status,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.users.Create(c.Request.Context(), u); err != nil {
		if store.IsUniqueViolation(err) {
			Error(c, http.StatusConflict, "username_taken", "username is already taken")
			return
		}
		Error(c, http.StatusInternalServerError, "internal", "failed to create user")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user": publicUser(u)})
}

func (s *Service) login(c *gin.Context) {
	if !s.limiter.allow(c.ClientIP()) {
		Error(c, http.StatusTooManyRequests, "rate_limited", "too many attempts, try again later")
		return
	}
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	u, err := s.users.GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
		CheckPassword(dummyHash, req.Password) // uniform timing
		Error(c, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}
	if !CheckPassword(u.PasswordHash, req.Password) {
		Error(c, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}
	switch u.Status {
	case store.StatusPending:
		Error(c, http.StatusForbidden, "account_pending", "account is waiting for admin approval")
		return
	case store.StatusDisabled:
		Error(c, http.StatusForbidden, "account_disabled", "account is disabled")
		return
	}

	if err := s.createSession(c, u.ID); err != nil {
		Error(c, http.StatusInternalServerError, "internal", "failed to create session")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": publicUser(u)})
}

func (s *Service) logout(c *gin.Context) {
	if token, err := c.Cookie(SessionCookieName); err == nil {
		_ = s.sessions.DeleteByTokenHash(c.Request.Context(), hashToken(token))
	}
	s.clearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Service) me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user": publicUser(CurrentUser(c))})
}

func (s *Service) registrationMode(c *gin.Context) string {
	mode, err := s.settings.Get(c.Request.Context(), settingRegMode)
	if err != nil {
		return regModeOpen
	}
	switch mode {
	case regModeOpen, regModeApproval, regModeClosed:
		return mode
	default:
		return regModeOpen
	}
}

func (s *Service) createSession(c *gin.Context, userID string) error {
	token, err := newToken()
	if err != nil {
		return err
	}
	now := store.Now()
	sess := &store.Session{
		ID:        store.NewID(),
		UserID:    userID,
		TokenHash: hashToken(token),
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		ExpiresAt: now.Add(sessionTTL),
		CreatedAt: now,
	}
	if err := s.sessions.Create(c.Request.Context(), sess); err != nil {
		return err
	}
	s.setSessionCookie(c, token)
	return nil
}

func (s *Service) setSessionCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, token, int(sessionTTL.Seconds()), "/", "",
		c.Request.TLS != nil, true)
}

func (s *Service) clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, "", -1, "/", "", c.Request.TLS != nil, true)
}

func newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func publicUser(u *store.User) gin.H {
	return UserJSON(u)
}

// UserJSON is the public user representation used across endpoints.
func UserJSON(u *store.User) gin.H {
	return gin.H{
		"id":         u.ID,
		"username":   u.Username,
		"nickname":   u.Nickname,
		"role":       u.Role,
		"is_founder": u.IsFounder,
		"status":     u.Status,
		"has_avatar": u.HasAvatar,
		"bio":        u.Bio,
		"created_at": u.CreatedAt,
		"updated_at": u.UpdatedAt,
	}
}

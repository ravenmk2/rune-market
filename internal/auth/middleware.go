package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ravenmk2/rune-market/internal/store"
)

// Resolve is optional-auth middleware: it populates the context with the
// current user and session when a valid session cookie is present, and
// slides the expiry forward when more than half the TTL has elapsed.
func (s *Service) Resolve() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(SessionCookieName)
		if err != nil || token == "" {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		sess, err := s.sessions.GetByTokenHash(ctx, hashToken(token))
		if err != nil {
			c.Next()
			return
		}
		if time.Now().After(sess.ExpiresAt) {
			_ = s.sessions.DeleteByTokenHash(ctx, sess.TokenHash)
			s.clearSessionCookie(c)
			c.Next()
			return
		}
		u, err := s.users.GetByID(ctx, sess.UserID)
		if err != nil || u.Status != store.StatusActive {
			c.Next()
			return
		}
		if time.Until(sess.ExpiresAt) < renewBelow {
			sess.ExpiresAt = store.Now().Add(sessionTTL)
			_ = s.sessions.Touch(ctx, sess.ID, sess.ExpiresAt)
			s.setSessionCookie(c, token)
		}
		c.Set(ctxUserKey, u)
		c.Set(ctxSessionKey, sess)
		c.Next()
	}
}

// RequireAuth aborts with 401 unless Resolve authenticated the request.
func (s *Service) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentUser(c) == nil {
			Error(c, http.StatusUnauthorized, "unauthenticated", "login required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireAdmin aborts with 401/403 unless the current user is an admin.
func (s *Service) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			Error(c, http.StatusUnauthorized, "unauthenticated", "login required")
			c.Abort()
			return
		}
		if u.Role != store.RoleAdmin {
			Error(c, http.StatusForbidden, "forbidden", "admin required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// CSRFProtect rejects state-changing requests without the custom
// X-Requested-With header (design §7; SameSite=Lax is the second layer).
func CSRFProtect() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if c.GetHeader("X-Requested-With") != "XMLHttpRequest" {
			Error(c, http.StatusForbidden, "csrf_failed", "missing X-Requested-With header")
			c.Abort()
			return
		}
		c.Next()
	}
}

// CurrentUser returns the authenticated user or nil.
func CurrentUser(c *gin.Context) *store.User {
	if v, ok := c.Get(ctxUserKey); ok {
		if u, ok := v.(*store.User); ok {
			return u
		}
	}
	return nil
}

// CurrentSession returns the authenticated session or nil.
func CurrentSession(c *gin.Context) *store.Session {
	if v, ok := c.Get(ctxSessionKey); ok {
		if sess, ok := v.(*store.Session); ok {
			return sess
		}
	}
	return nil
}

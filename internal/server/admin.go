package server

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/hub"
	"github.com/ravenmk2/rune-market/internal/secret"
	"github.com/ravenmk2/rune-market/internal/store"
)

// AdminHandler serves the §8.5 admin panel endpoints. All routes sit under
// RequireAdmin; founder/self protections are enforced per endpoint.
type AdminHandler struct {
	apiSettings
	stores  *store.Stores
	skills  *hub.Skills
	designs *hub.Designs
	blobs   *blob.Storage
	logger  *logrus.Logger
	version string
	dataDir string
	dialect string
}

func NewAdminHandler(stores *store.Stores, skills *hub.Skills, designs *hub.Designs,
	blobs *blob.Storage, logger *logrus.Logger, version, dataDir, dialect string) *AdminHandler {
	return &AdminHandler{
		apiSettings: apiSettings{settings: stores.Settings},
		stores:      stores,
		skills:      skills, designs: designs, blobs: blobs,
		logger: logger, version: version, dataDir: dataDir, dialect: dialect,
	}
}

// RegisterRoutes mounts §8.5 routes on an admin-gated group.
func (h *AdminHandler) RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/overview", h.overview)

	g.GET("/users", h.listUsers)
	g.POST("/users", h.createUser)
	g.POST("/users/:id/approve", h.approveUser)
	g.POST("/users/:id/disable", h.disableUser)
	g.POST("/users/:id/enable", h.enableUser)
	g.POST("/users/:id/role", h.setUserRole)
	g.POST("/users/:id/reset-password", h.resetPassword)
	g.DELETE("/users/:id", h.deleteUser)

	g.GET("/skills", h.listSkills)
	g.POST("/skills/:id/official", h.skillOfficial)
	g.POST("/skills/:id/unofficial", h.skillUnofficial)
	g.POST("/skills/:id/approve", h.approveSkill)

	g.GET("/designs", h.listDesigns)
	g.POST("/designs/:id/official", h.designOfficial)
	g.POST("/designs/:id/unofficial", h.designUnofficial)
	g.POST("/designs/:id/approve", h.approveDesign)

	g.GET("/settings", h.getSettings)
	g.PUT("/settings", h.putSettings)
	g.POST("/settings/secret/regenerate", h.regenerateSecret)
}

// --- overview ---

func (h *AdminHandler) overview(c *gin.Context) {
	ctx := c.Request.Context()
	o, err := store.NewStatsStore(h.stores.DB).Overview(ctx)
	if err != nil {
		h.logger.WithError(err).Error("admin: overview failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to gather stats")
		return
	}
	// §6.3: avatars are user-addressed and scanned on demand
	avatarBytes, err := h.blobs.AvatarBytes()
	if err != nil {
		h.logger.WithError(err).Error("admin: avatar scan failed")
	}
	_, secretErr := os.Stat(filepath.Join(h.dataDir, "secret"))
	c.JSON(http.StatusOK, gin.H{
		"stats": gin.H{
			"users": o.Users, "skills": o.Skills, "designs": o.Designs,
			"storage_bytes": o.StorageBytes + avatarBytes,
		},
		"todos": gin.H{
			"pending_users": o.PendingUsers, "pending_skills": o.PendingSkills,
			"pending_designs": o.PendingDesigns,
		},
		"system": gin.H{
			"version": h.version, "database": h.dialect,
			"registration_mode": h.getStr(ctx, "registration_mode", "open"),
			"data_dir":          h.dataDir,
			"secret_created":    secretErr == nil,
		},
	})
}

// --- users ---

func adminUserJSON(u *store.User) gin.H {
	return gin.H{
		"id": u.ID, "username": u.Username, "nickname": u.Nickname,
		"role": u.Role, "is_founder": u.IsFounder, "status": u.Status,
		"has_avatar": u.HasAvatar, "created_at": u.CreatedAt,
	}
}

func (h *AdminHandler) listUsers(c *gin.Context) {
	ctx := c.Request.Context()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := h.getInt(ctx, "page_size", 20)
	status := c.Query("status")
	switch status {
	case "", store.StatusActive, store.StatusPending, store.StatusDisabled:
	default:
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "unknown status filter")
		return
	}
	users, total, err := h.stores.Users.List(ctx, store.UserFilter{
		Status: status, Query: c.Query("q"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to list users")
		return
	}
	out := make([]gin.H, 0, len(users))
	for _, u := range users {
		out = append(out, adminUserJSON(u))
	}
	c.JSON(http.StatusOK, gin.H{
		"items": out, "total": total, "page": page, "page_size": pageSize,
	})
}

type createUserRequest struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (h *AdminHandler) createUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if req.Role != store.RoleAdmin && req.Role != store.RoleUser {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "role must be admin or user")
		return
	}
	if details := auth.ValidateCredentials(req.Username, req.Nickname, req.Password); len(details) > 0 {
		auth.ErrorDetails(c, http.StatusBadRequest, "invalid_argument", "validation failed", details)
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to hash password")
		return
	}
	now := store.Now()
	u := &store.User{
		ID: store.NewID(), Username: req.Username, Nickname: req.Nickname,
		PasswordHash: hash, Role: req.Role, Status: store.StatusActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := h.stores.Users.Create(c.Request.Context(), u); err != nil {
		if store.IsUniqueViolation(err) {
			auth.Error(c, http.StatusConflict, "username_taken", "username is already taken")
			return
		}
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to create user")
		return
	}
	h.logger.WithFields(logrus.Fields{
		"actor": auth.CurrentUser(c).Username, "target": u.Username, "role": u.Role,
	}).Info("admin: user created")
	c.JSON(http.StatusCreated, gin.H{"user": adminUserJSON(u)})
}

// loadTarget resolves the target user or writes the error response.
func (h *AdminHandler) loadTarget(c *gin.Context) *store.User {
	u, err := h.stores.Users.GetByID(c.Request.Context(), c.Param("id"))
	if errors.Is(err, store.ErrNotFound) {
		auth.Error(c, http.StatusNotFound, "not_found", "user not found")
		return nil
	}
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load user")
		return nil
	}
	return u
}

func (h *AdminHandler) approveUser(c *gin.Context) {
	target := h.loadTarget(c)
	if target == nil {
		return
	}
	if target.Status != store.StatusPending {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "user is not pending")
		return
	}
	h.setUserStatus(c, target, store.StatusActive)
}

func (h *AdminHandler) disableUser(c *gin.Context) {
	target := h.loadTarget(c)
	if target == nil || !h.guardMutation(c, target, "disable") {
		return
	}
	if target.Status == store.StatusDisabled {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "user is already disabled")
		return
	}
	// disabling also invalidates the user's sessions
	_ = h.stores.Sessions.DeleteByUserID(c.Request.Context(), target.ID)
	h.setUserStatus(c, target, store.StatusDisabled)
}

func (h *AdminHandler) enableUser(c *gin.Context) {
	target := h.loadTarget(c)
	if target == nil {
		return
	}
	if target.Status != store.StatusDisabled {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "user is not disabled")
		return
	}
	h.setUserStatus(c, target, store.StatusActive)
}

func (h *AdminHandler) setUserStatus(c *gin.Context, target *store.User, status string) {
	if err := h.stores.Users.UpdateStatus(c.Request.Context(), target.ID, status, store.Now()); err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to update user")
		return
	}
	target.Status = status
	c.JSON(http.StatusOK, gin.H{"user": adminUserJSON(target)})
}

// guardMutation enforces founder/self protections (§7); writes the 403
// response and returns false when the mutation is not allowed.
func (h *AdminHandler) guardMutation(c *gin.Context, target *store.User, action string) bool {
	if err := auth.EnsureMutable(target); err != nil {
		auth.Error(c, http.StatusForbidden, "founder_protected",
			"founder account cannot be "+action)
		return false
	}
	if target.ID == auth.CurrentUser(c).ID {
		auth.Error(c, http.StatusForbidden, "self_protected",
			"cannot "+action+" your own account")
		return false
	}
	return true
}

type setRoleRequest struct {
	Role string `json:"role"`
}

func (h *AdminHandler) setUserRole(c *gin.Context) {
	target := h.loadTarget(c)
	if target == nil {
		return
	}
	var req setRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if req.Role != store.RoleAdmin && req.Role != store.RoleUser {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "role must be admin or user")
		return
	}
	// demotions are protected; promoting to admin is always fine
	if req.Role == store.RoleUser && !h.guardMutation(c, target, "demote") {
		return
	}
	if err := h.stores.Users.UpdateRole(c.Request.Context(), target.ID, req.Role, store.Now()); err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to update role")
		return
	}
	h.logger.WithFields(logrus.Fields{
		"actor": auth.CurrentUser(c).Username, "target": target.Username, "role": req.Role,
	}).Info("admin: role changed")
	target.Role = req.Role
	c.JSON(http.StatusOK, gin.H{"user": adminUserJSON(target)})
}

// tempPasswordAlphabet avoids ambiguous characters (no 0/O, 1/l/I).
const tempPasswordAlphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"

func tempPassword(n int) (string, error) {
	out := make([]byte, n)
	max := big.NewInt(int64(len(tempPasswordAlphabet)))
	for i := range out {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = tempPasswordAlphabet[v.Int64()]
	}
	return string(out), nil
}

func (h *AdminHandler) resetPassword(c *gin.Context) {
	target := h.loadTarget(c)
	if target == nil {
		return
	}
	pw, err := tempPassword(12)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to generate password")
		return
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to hash password")
		return
	}
	ctx := c.Request.Context()
	if err := h.stores.Users.UpdatePassword(ctx, target.ID, hash, store.Now()); err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to reset password")
		return
	}
	// the temporary password shows once; existing sessions are invalidated
	_ = h.stores.Sessions.DeleteByUserID(ctx, target.ID)
	h.logger.WithFields(logrus.Fields{
		"actor": auth.CurrentUser(c).Username, "target": target.Username,
	}).Info("admin: password reset")
	c.JSON(http.StatusOK, gin.H{"temporary_password": pw})
}

// deleteUser removes the account. Artifacts are taken down but kept per
// §8.5, which leaves their owner_id dangling: the owner FK has no ON
// DELETE action, so enforcement is paused on a dedicated connection for
// this single archival delete (PRAGMA cannot change inside a tx).
func (h *AdminHandler) deleteUser(c *gin.Context) {
	target := h.loadTarget(c)
	if target == nil || !h.guardMutation(c, target, "delete") {
		return
	}
	ctx := c.Request.Context()
	if err := deleteUserAccount(ctx, h.stores, h.dialect, h.dataDir, target.ID); err != nil {
		h.logger.WithError(err).Error("admin: delete user failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to delete user")
		return
	}
	h.logger.WithFields(logrus.Fields{
		"actor": auth.CurrentUser(c).Username, "target": target.Username,
	}).Info("admin: user deleted")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// deleteUserAccount removes an account the §8.5 way: artifacts cascade to
// taken_down (kept, so their owner_id dangles — the owner FK has no ON
// DELETE action, therefore enforcement is paused on a dedicated
// connection for this archival delete), sessions are removed explicitly,
// and the avatar file group is deleted.
func deleteUserAccount(ctx context.Context, stores *store.Stores, dialect, dataDir, targetID string) error {
	conn, err := stores.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	fkOff, fkOn := `PRAGMA foreign_keys = OFF`, `PRAGMA foreign_keys = ON`
	if dialect == store.DialectMySQL {
		fkOff, fkOn = `SET FOREIGN_KEY_CHECKS = 0`, `SET FOREIGN_KEY_CHECKS = 1`
	}
	if _, err := conn.ExecContext(ctx, fkOff); err != nil {
		return err
	}
	// never leave a pooled connection with checks disabled
	defer func() { _, _ = conn.ExecContext(ctx, fkOn) }()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := store.Now()
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE skill SET status = ?, updated_at = ? WHERE owner_id = ? AND status = ?`,
			[]any{store.SkillStatusTakenDown, now, targetID, store.SkillStatusPublished}},
		{`UPDATE designmd SET status = ?, updated_at = ? WHERE owner_id = ? AND status = ?`,
			[]any{store.DesignStatusTakenDown, now, targetID, store.DesignStatusPublished}},
		// FK enforcement is paused, so the session cascade will not fire;
		// remove sessions explicitly
		{`DELETE FROM session WHERE user_id = ?`, []any{targetID}},
	} {
		if _, err := tx.ExecContext(ctx, stmt.sql, stmt.args...); err != nil {
			return err
		}
	}
	if err := store.NewUserStore(tx).Delete(ctx, targetID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return deleteAvatarFiles(dataDir, targetID)
}

// deleteAvatarFiles removes the avatar file group (§11).
func deleteAvatarFiles(dataDir, userID string) error {
	matches, _ := filepath.Glob(filepath.Join(dataDir, "avatars", userID+"*.png"))
	for _, m := range matches {
		_ = os.Remove(m)
	}
	return nil
}

// --- artifacts ---

func (h *AdminHandler) listSkills(c *gin.Context) {
	ctx := c.Request.Context()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := h.getInt(ctx, "page_size", 20)
	items, total, err := h.skills.List(ctx, hub.ListFilter{
		Status:   c.Query("status"),
		Official: c.Query("official") == "true" || c.Query("official") == "1",
		Page:     page, PageSize: pageSize,
	})
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to list skills")
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, skillItemJSON(it))
	}
	c.JSON(http.StatusOK, gin.H{
		"items": out, "total": total, "page": page, "page_size": pageSize,
	})
}

func (h *AdminHandler) listDesigns(c *gin.Context) {
	ctx := c.Request.Context()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := h.getInt(ctx, "page_size", 20)
	items, total, err := h.designs.List(ctx, hub.ListFilter{
		Status:   c.Query("status"),
		Official: c.Query("official") == "true" || c.Query("official") == "1",
		Page:     page, PageSize: pageSize,
	})
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to list designs")
		return
	}
	out := make([]gin.H, 0, len(items))
	for _, it := range items {
		out = append(out, designItemJSON(it))
	}
	c.JSON(http.StatusOK, gin.H{
		"items": out, "total": total, "page": page, "page_size": pageSize,
	})
}

func (h *AdminHandler) skillOfficial(c *gin.Context)   { h.setSkillOfficial(c, true) }
func (h *AdminHandler) skillUnofficial(c *gin.Context) { h.setSkillOfficial(c, false) }

func (h *AdminHandler) setSkillOfficial(c *gin.Context, official bool) {
	sk, err := h.skills.SetOfficial(c.Request.Context(), c.Param("id"), official)
	switch {
	case errors.Is(err, hub.ErrNotFound):
		auth.Error(c, http.StatusNotFound, "not_found", "skill not found")
	case err != nil:
		auth.Error(c, http.StatusInternalServerError, "internal", "operation failed")
	default:
		h.logger.WithFields(logrus.Fields{
			"actor": auth.CurrentUser(c).Username, "skill": sk.Name, "official": official,
		}).Info("admin: skill official flag changed")
		d, err := h.skills.GetDetailByID(c.Request.Context(), sk.ID)
		if err != nil {
			auth.Error(c, http.StatusInternalServerError, "internal", "failed to load skill")
			return
		}
		c.JSON(http.StatusOK, gin.H{"skill": skillDetailJSON(d)})
	}
}

func (h *AdminHandler) approveSkill(c *gin.Context) {
	sk, err := h.skills.ApproveSkill(c.Request.Context(), c.Param("id"))
	switch {
	case errors.Is(err, hub.ErrNotFound):
		auth.Error(c, http.StatusNotFound, "not_found", "skill not found")
	case errors.Is(err, hub.ErrInvalidInput):
		auth.Error(c, http.StatusBadRequest, "invalid_argument", err.Error())
	case err != nil:
		auth.Error(c, http.StatusInternalServerError, "internal", "operation failed")
	default:
		h.logger.WithFields(logrus.Fields{
			"actor": auth.CurrentUser(c).Username, "skill": sk.Name,
		}).Info("admin: skill approved")
		d, err := h.skills.GetDetailByID(c.Request.Context(), sk.ID)
		if err != nil {
			auth.Error(c, http.StatusInternalServerError, "internal", "failed to load skill")
			return
		}
		c.JSON(http.StatusOK, gin.H{"skill": skillDetailJSON(d)})
	}
}

func (h *AdminHandler) designOfficial(c *gin.Context)   { h.setDesignOfficial(c, true) }
func (h *AdminHandler) designUnofficial(c *gin.Context) { h.setDesignOfficial(c, false) }

func (h *AdminHandler) setDesignOfficial(c *gin.Context, official bool) {
	d, err := h.designs.SetOfficial(c.Request.Context(), c.Param("id"), official)
	switch {
	case errors.Is(err, hub.ErrNotFound):
		auth.Error(c, http.StatusNotFound, "not_found", "design not found")
	case err != nil:
		auth.Error(c, http.StatusInternalServerError, "internal", "operation failed")
	default:
		h.logger.WithFields(logrus.Fields{
			"actor": auth.CurrentUser(c).Username, "design": d.Name, "official": official,
		}).Info("admin: design official flag changed")
		det, err := h.designs.GetDetailByID(c.Request.Context(), d.ID)
		if err != nil {
			auth.Error(c, http.StatusInternalServerError, "internal", "failed to load design")
			return
		}
		exts, err := previewExtMap(c.Request.Context(), h.blobs, h.stores.DB, det.Latest)
		if err != nil {
			auth.Error(c, http.StatusInternalServerError, "internal", "failed to load design")
			return
		}
		c.JSON(http.StatusOK, gin.H{"design": designDetailJSON(det, exts)})
	}
}

func (h *AdminHandler) approveDesign(c *gin.Context) {
	d, err := h.designs.ApproveDesign(c.Request.Context(), c.Param("id"))
	switch {
	case errors.Is(err, hub.ErrNotFound):
		auth.Error(c, http.StatusNotFound, "not_found", "design not found")
	case errors.Is(err, hub.ErrInvalidInput):
		auth.Error(c, http.StatusBadRequest, "invalid_argument", err.Error())
	case err != nil:
		auth.Error(c, http.StatusInternalServerError, "internal", "operation failed")
	default:
		h.logger.WithFields(logrus.Fields{
			"actor": auth.CurrentUser(c).Username, "design": d.Name,
		}).Info("admin: design approved")
		det, err := h.designs.GetDetailByID(c.Request.Context(), d.ID)
		if err != nil {
			auth.Error(c, http.StatusInternalServerError, "internal", "failed to load design")
			return
		}
		exts, err := previewExtMap(c.Request.Context(), h.blobs, h.stores.DB, det.Latest)
		if err != nil {
			auth.Error(c, http.StatusInternalServerError, "internal", "failed to load design")
			return
		}
		c.JSON(http.StatusOK, gin.H{"design": designDetailJSON(det, exts)})
	}
}

// --- settings ---

// settingKeys is the §6.3 key set exposed to admins, in stable order.
var settingKeys = []string{
	"site_name", "site_description", "site_tagline", "page_size", "registration_mode",
	"artifact_review", "upload_max_mb", "anonymous_browse", "anonymous_download",
}

func (h *AdminHandler) getSettings(c *gin.Context) {
	all, err := h.stores.Settings.GetAll(c.Request.Context())
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load settings")
		return
	}
	out := gin.H{}
	for _, k := range settingKeys {
		v, ok := all[k]
		if !ok {
			v = defaultSettings[k]
		}
		out[k] = v
	}
	c.JSON(http.StatusOK, gin.H{"settings": out})
}

// validateSetting checks one updated value; returns the error message.
func validateSetting(key, value string) string {
	switch key {
	case "site_name":
		if n := utf8.RuneCountInString(value); n < 1 || n > 64 {
			return "site_name must be 1-64 characters"
		}
	case "site_description":
		if utf8.RuneCountInString(value) > 256 {
			return "site_description must be at most 256 characters"
		}
	case "site_tagline":
		if utf8.RuneCountInString(value) > 200 {
			return "site_tagline must be at most 200 characters"
		}
	case "page_size":
		if n, err := strconv.Atoi(value); err != nil || n < 1 || n > 100 {
			return "page_size must be an integer in 1-100"
		}
	case "registration_mode":
		if value != "open" && value != "approval" && value != "closed" {
			return "registration_mode must be open|approval|closed"
		}
	case "artifact_review":
		if value != "none" && value != "required" {
			return "artifact_review must be none|required"
		}
	case "upload_max_mb":
		if n, err := strconv.Atoi(value); err != nil || n < 1 || n > 200 {
			return "upload_max_mb must be an integer in 1-200"
		}
	case "anonymous_browse", "anonymous_download":
		if value != "true" && value != "false" {
			return key + " must be true|false"
		}
	default:
		return "unknown setting key: " + key
	}
	return ""
}

func (h *AdminHandler) putSettings(c *gin.Context) {
	// Values may arrive as any JSON scalar (the SPA sends booleans and
	// numbers natively); normalize to the storage string form.
	var req struct {
		Settings map[string]any `json:"settings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if len(req.Settings) == 0 {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "no settings to update")
		return
	}
	updates := map[string]string{}
	var details []string
	for k, v := range req.Settings {
		s, ok := scalarString(v)
		if !ok {
			details = append(details, k+": value must be a string, number or boolean")
			continue
		}
		if msg := validateSetting(k, s); msg != "" {
			details = append(details, msg)
			continue
		}
		updates[k] = s
	}
	if len(details) > 0 {
		auth.ErrorDetails(c, http.StatusBadRequest, "invalid_argument", "validation failed", details)
		return
	}
	ctx := c.Request.Context()
	for k, v := range updates {
		if err := h.stores.Settings.Set(ctx, k, v); err != nil {
			auth.Error(c, http.StatusInternalServerError, "internal", "failed to save settings")
			return
		}
	}
	h.getSettings(c)
}

// scalarString normalizes a JSON scalar to its canonical string form.
func scalarString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case bool:
		return strconv.FormatBool(t), true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	default:
		return "", false
	}
}

// regenerateSecret rewrites ./data/secret (0600) and invalidates every
// session (§13; the admin UI marks this action as dangerous).
func (h *AdminHandler) regenerateSecret(c *gin.Context) {
	if _, err := secret.Regenerate(h.dataDir); err != nil {
		h.logger.WithError(err).Error("admin: secret regenerate failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to regenerate secret")
		return
	}
	if err := h.stores.Sessions.DeleteAll(c.Request.Context()); err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to invalidate sessions")
		return
	}
	h.logger.WithFields(logrus.Fields{
		"actor": auth.CurrentUser(c).Username,
	}).Info("admin: master secret regenerated, all sessions invalidated")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

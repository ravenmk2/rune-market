package server

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/hub"
	"github.com/ravenmk2/rune-market/internal/store"
)

// AccountHandler serves the §8.2 account endpoints (avatar, account
// deletion), the public user profile (§8.2) and the /avatars file route.
type AccountHandler struct {
	apiSettings
	stores  *store.Stores
	skills  *hub.Skills
	designs *hub.Designs
	blobs   *blob.Storage
	logger  *logrus.Logger
	dataDir string
}

func NewAccountHandler(stores *store.Stores, skills *hub.Skills, designs *hub.Designs,
	blobs *blob.Storage, logger *logrus.Logger, dataDir string) *AccountHandler {
	return &AccountHandler{
		apiSettings: apiSettings{settings: stores.Settings},
		stores:      stores, skills: skills, designs: designs,
		blobs: blobs, logger: logger, dataDir: dataDir,
	}
}

func (h *AccountHandler) RegisterRoutes(r *gin.Engine, api, apiAuth *gin.RouterGroup) {
	r.GET("/avatars/:name", h.serveAvatar)
	api.GET("/users/:username", h.profile)
	apiAuth.POST("/account/avatar", h.uploadAvatar)
	apiAuth.DELETE("/account/avatar", h.deleteAvatar)
	apiAuth.DELETE("/account", h.deleteAccount)
}

// --- avatar (§8.2, §11 user-addressed model) ---

func (h *AccountHandler) uploadAvatar(c *gin.Context) {
	u := auth.CurrentUser(c)
	if err := h.blobs.PutAvatar(u.ID, c.Request.Body); err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_image", err.Error())
		return
	}
	now := store.Now()
	if err := h.stores.Users.UpdateAvatar(c.Request.Context(), u.ID, true, now); err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to update user")
		return
	}
	u.HasAvatar = true
	u.UpdatedAt = now // the SPA busts avatar caches via ?v=updated_at
	c.JSON(http.StatusOK, gin.H{"user": auth.UserJSON(u)})
}

func (h *AccountHandler) deleteAvatar(c *gin.Context) {
	u := auth.CurrentUser(c)
	if err := h.blobs.DeleteAvatar(u.ID); err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to delete avatar")
		return
	}
	now := store.Now()
	if err := h.stores.Users.UpdateAvatar(c.Request.Context(), u.ID, false, now); err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to update user")
		return
	}
	u.HasAvatar = false
	u.UpdatedAt = now
	c.JSON(http.StatusOK, gin.H{"user": auth.UserJSON(u)})
}

// avatarNameRe validates /avatars/<user_id>[_size].png requests.
var avatarNameRe = regexp.MustCompile(`^([0-9a-f]{32})(_(32|64|128))?\.png$`)

func (h *AccountHandler) serveAvatar(c *gin.Context) {
	m := avatarNameRe.FindStringSubmatch(c.Param("name"))
	if m == nil {
		auth.Error(c, http.StatusNotFound, "not_found", "avatar not found")
		return
	}
	// short cache; ?v=<updated_at> busts it on change (§11)
	c.Header("Cache-Control", "public, max-age=60")
	p := h.blobs.AvatarPath(m[1], m[3])
	c.File(p)
}

// --- public profile (§8.2) ---

func (h *AccountHandler) profile(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	ctx := c.Request.Context()
	u, err := h.stores.Users.GetByUsername(ctx, c.Param("username"))
	if errors.Is(err, store.ErrNotFound) {
		auth.Error(c, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load user")
		return
	}

	skills, _, err := h.skills.List(ctx, hub.ListFilter{
		OwnerID: u.ID, PublicOnly: true, PageSize: 1000,
	})
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to list skills")
		return
	}
	designs, _, err := h.designs.List(ctx, hub.ListFilter{
		OwnerID: u.ID, PublicOnly: true, PageSize: 1000,
	})
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to list designs")
		return
	}

	skillItems := make([]gin.H, 0, len(skills))
	for _, it := range skills {
		skillItems = append(skillItems, skillItemJSON(it))
	}
	designItems := make([]gin.H, 0, len(designs))
	for _, it := range designs {
		designItems = append(designItems, designItemJSON(it))
	}
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id": u.ID, "username": u.Username, "nickname": u.Nickname,
			"bio": u.Bio, "has_avatar": u.HasAvatar,
			"updated_at": u.UpdatedAt, "created_at": u.CreatedAt,
		},
		"skills":  skillItems,
		"designs": designItems,
	})
}

// --- account deletion (§8.2) ---

func (h *AccountHandler) deleteAccount(c *gin.Context) {
	u := auth.CurrentUser(c)
	if err := auth.EnsureMutable(u); err != nil {
		auth.Error(c, http.StatusForbidden, "founder_protected",
			"founder account cannot be deleted")
		return
	}
	ctx := c.Request.Context()
	if err := deleteUserAccount(ctx, h.stores, h.stores.Dialect, h.dataDir, u.ID); err != nil {
		h.logger.WithError(err).Error("account: self delete failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to delete account")
		return
	}
	h.logger.WithField("target", u.Username).Info("account: user self-deleted")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

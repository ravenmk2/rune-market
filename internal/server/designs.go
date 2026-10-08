package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/designmd"
	"github.com/ravenmk2/rune-market/internal/hub"
	"github.com/ravenmk2/rune-market/internal/store"
)

// DesignsHandler serves the §8.3/§8.4 design endpoints, POST /blobs and
// the /images file routes (§8.3, §11).
type DesignsHandler struct {
	apiSettings
	hub    *hub.Designs
	blobs  *blob.Storage
	logger *logrus.Logger
}

func NewDesignsHandler(h *hub.Designs, settings *store.SettingStore, blobs *blob.Storage, logger *logrus.Logger) *DesignsHandler {
	return &DesignsHandler{apiSettings: apiSettings{settings: settings}, hub: h, blobs: blobs, logger: logger}
}

// RegisterRoutes mounts public routes on api, authenticated routes on
// apiAuth, and the image file route on the engine root (outside /api).
func (h *DesignsHandler) RegisterRoutes(r *gin.Engine, api, apiAuth *gin.RouterGroup) {
	r.GET("/images/:name", h.serveImage)

	api.GET("/designs", h.list)
	api.GET("/designs/:ns/:name", h.detail)
	api.GET("/designs/:ns/:name/versions", h.versions)
	api.GET("/designs/:ns/:name/versions/:ver", h.version)
	api.GET("/designs/:ns/:name/versions/:ver/content", h.content)
	api.GET("/designs/:ns/:name/versions/:ver/download", h.download)

	apiAuth.POST("/blobs", h.uploadBlob)
	apiAuth.POST("/designs/validate", h.validate)
	apiAuth.POST("/designs", h.publish)
	apiAuth.GET("/mine/designs", h.mine)
	apiAuth.PUT("/designs/:id", h.update)
	apiAuth.POST("/designs/:id/takedown", h.takedown)
	apiAuth.POST("/designs/:id/restore", h.restore)
	apiAuth.DELETE("/designs/:id", h.delete)
}

// --- response shaping (contract shapes) ---

// previewURL maps a blob sha to its /images path (nil when absent).
func previewURL(sha *string, suffix string) any {
	if sha == nil || *sha == "" {
		return nil
	}
	return "/images/" + *sha + suffix + ".png"
}

func designItemJSON(it hub.DesignItem) gin.H {
	tags := it.Tags
	if tags == nil {
		tags = []string{}
	}
	return gin.H{
		"id":                it.Design.ID,
		"namespace":         it.OwnerUsername,
		"name":              it.Design.Name,
		"summary":           it.Design.Summary,
		"official":          it.Design.Official,
		"status":            it.Design.Status,
		"tags":              tags,
		"latest_version":    it.LatestVersion,
		"download_count":    it.Design.DownloadCount,
		"updated_at":        it.Design.UpdatedAt,
		"preview_thumb_url": previewURL(it.PreviewDesktopSHA256, "_640"),
		"owner": gin.H{
			"username": it.OwnerUsername,
			"nickname": it.OwnerNickname,
		},
	}
}

func designVersionJSON(v *store.DesignmdVersion) gin.H {
	return gin.H{
		"version":             v.Version,
		"sha256":              v.SHA256,
		"preview_desktop_url": previewURL(v.PreviewDesktopSHA256, ""),
		"preview_mobile_url":  previewURL(v.PreviewMobileSHA256, ""),
		"created_at":          v.CreatedAt,
	}
}

func designDetailJSON(d *hub.DesignDetail) gin.H {
	out := designItemJSON(d.DesignItem)
	if d.Latest != nil {
		out["latest"] = designVersionJSON(d.Latest)
	}
	return out
}

// --- public read endpoints ---

func (h *DesignsHandler) list(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	ctx := c.Request.Context()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := h.getInt(ctx, "page_size", 20)
	items, total, err := h.hub.List(ctx, hub.ListFilter{
		Query:      c.Query("q"),
		Tag:        c.Query("tag"),
		Official:   c.Query("official") == "true" || c.Query("official") == "1",
		Sort:       c.Query("sort"),
		Page:       page,
		PageSize:   pageSize,
		PublicOnly: true,
	})
	if err != nil {
		h.logger.WithError(err).Error("designs: list failed")
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

// visibleDetail enforces that non-published designs are only visible to
// their owner or an admin (404 to others), symmetric with skills.
func (h *DesignsHandler) visibleDetail(c *gin.Context, ns, name string) *hub.DesignDetail {
	d, err := h.hub.GetDetail(c.Request.Context(), ns, name)
	if errors.Is(err, hub.ErrNotFound) {
		auth.Error(c, http.StatusNotFound, "not_found", "design not found")
		return nil
	}
	if err != nil {
		h.logger.WithError(err).Error("designs: detail failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load design")
		return nil
	}
	if d.Design.Status != store.DesignStatusPublished {
		u := auth.CurrentUser(c)
		if u == nil || (u.ID != d.Design.OwnerID && u.Role != store.RoleAdmin) {
			auth.Error(c, http.StatusNotFound, "not_found", "design not found")
			return nil
		}
	}
	return d
}

func (h *DesignsHandler) detail(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	d := h.visibleDetail(c, c.Param("ns"), c.Param("name"))
	if d == nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{"design": designDetailJSON(d)})
}

func (h *DesignsHandler) versions(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	d := h.visibleDetail(c, c.Param("ns"), c.Param("name"))
	if d == nil {
		return
	}
	versions, err := h.hub.ListVersions(c.Request.Context(), d.Design.ID)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to list versions")
		return
	}
	out := make([]gin.H, 0, len(versions))
	for _, v := range versions {
		out = append(out, designVersionJSON(v))
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// visibleVersion resolves :ns/:name/:ver with visibility checks.
func (h *DesignsHandler) visibleVersion(c *gin.Context) (*hub.DesignDetail, *store.DesignmdVersion) {
	d := h.visibleDetail(c, c.Param("ns"), c.Param("name"))
	if d == nil {
		return nil, nil
	}
	v, err := h.hub.GetVersion(c.Request.Context(), d.Design.ID, c.Param("ver"))
	if errors.Is(err, hub.ErrNotFound) {
		auth.Error(c, http.StatusNotFound, "not_found", "version not found")
		return nil, nil
	}
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load version")
		return nil, nil
	}
	return d, v
}

func (h *DesignsHandler) version(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	_, v := h.visibleVersion(c)
	if v == nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": designVersionJSON(v)})
}

func (h *DesignsHandler) content(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	_, v := h.visibleVersion(c)
	if v == nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"content": v.Content, "sha256": v.SHA256, "version": v.Version,
	})
}

func (h *DesignsHandler) download(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	if !h.getBool(c.Request.Context(), "anonymous_download", true) &&
		auth.CurrentUser(c) == nil {
		auth.Error(c, http.StatusUnauthorized, "unauthenticated", "login required")
		return
	}
	d, v := h.visibleVersion(c)
	if v == nil {
		return
	}
	h.hub.IncrDownload(c.Request.Context(), d.Design.ID)
	filename := fmt.Sprintf("%s-%s.md", d.Design.Name, v.Version)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(v.Content))
}

// --- images (§8.3, §11: immutable cache) ---

var imageNameRe = regexp.MustCompile(`^[0-9a-f]{64}(_640)?\.png$`)

func (h *DesignsHandler) serveImage(c *gin.Context) {
	name := c.Param("name")
	if !imageNameRe.MatchString(name) {
		auth.Error(c, http.StatusNotFound, "not_found", "image not found")
		return
	}
	dir, err := h.blobsDir()
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "image storage unavailable")
		return
	}
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err != nil {
		auth.Error(c, http.StatusNotFound, "not_found", "image not found")
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.File(p)
}

func (h *DesignsHandler) blobsDir() (string, error) {
	p, err := h.blobs.Path(blob.KindImage, "x")
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}

// --- upload endpoints ---

// uploadBlob handles POST /blobs: raw body = PNG/JPG image, normalized to
// PNG with content addressing (§8.4, §11).
func (h *DesignsHandler) uploadBlob(c *gin.Context) {
	sum, size, err := h.blobs.PutImage(c.Request.Context(), h.hubDB(), c.Request.Body)
	if err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_image", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"sha256": sum, "size": size})
}

// readMarkdownBody reads the raw .md body with the 1MB cap (§10.1).
func (h *DesignsHandler) readMarkdownBody(c *gin.Context) ([]byte, bool) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, designmd.MaxContentBytes+1))
	if err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_upload", "failed to read body")
		return nil, false
	}
	if int64(len(body)) > designmd.MaxContentBytes {
		auth.Error(c, http.StatusRequestEntityTooLarge, "invalid_upload", "content exceeds 1MB limit")
		return nil, false
	}
	return body, true
}

func (h *DesignsHandler) validate(c *gin.Context) {
	body, ok := h.readMarkdownBody(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, designmd.Validate(body))
}

func (h *DesignsHandler) publish(c *gin.Context) {
	q := c.Request.URL.Query()
	name := q.Get("name")
	if name == "" {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "name query param is required")
		return
	}
	version := q.Get("version")
	if version == "" {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "version query param is required")
		return
	}
	body, ok := h.readMarkdownBody(c)
	if !ok {
		return
	}
	if report := designmd.Validate(body); report.HasErrors() {
		var details []string
		for _, chk := range report.Checks {
			if chk.Level == designmd.LevelError {
				details = append(details, chk.Title+": "+chk.Detail)
			}
		}
		auth.ErrorDetails(c, http.StatusBadRequest, "invalid_content", "content validation failed", details)
		return
	}

	ctx := c.Request.Context()
	user := auth.CurrentUser(c)
	d, _, err := h.hub.Publish(ctx, hub.DesignPublishInput{
		Owner:          user,
		Name:           name,
		Summary:        q.Get("summary"),
		Version:        version,
		Tags:           parseTags(q.Get("tags")),
		Content:        body,
		PreviewDesktop: q.Get("preview_desktop"),
		PreviewMobile:  q.Get("preview_mobile"),
		ReviewRequired: h.getStr(ctx, "artifact_review", "none") == "required",
		Official:       user.Role == store.RoleAdmin,
	})
	switch {
	case errors.Is(err, hub.ErrVersionExists):
		auth.Error(c, http.StatusConflict, "version_exists", "version already published")
		return
	case errors.Is(err, hub.ErrInvalidInput):
		auth.Error(c, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	case err != nil:
		h.logger.WithError(err).Error("designs: publish failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to publish")
		return
	}

	det, err := h.hub.GetDetail(ctx, user.Username, d.Name)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load design")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"design": designDetailJSON(det)})
}

// --- owner endpoints ---

func (h *DesignsHandler) mine(c *gin.Context) {
	ctx := c.Request.Context()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := h.getInt(ctx, "page_size", 20)
	items, total, err := h.hub.List(ctx, hub.ListFilter{
		OwnerID:  auth.CurrentUser(c).ID,
		Page:     page,
		PageSize: pageSize,
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

type updateDesignRequest struct {
	Summary *string  `json:"summary"`
	Tags    []string `json:"tags"`
}

func (h *DesignsHandler) update(c *gin.Context) {
	var req updateDesignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	err := h.hub.Update(c.Request.Context(), auth.CurrentUser(c), c.Param("id"), req.Summary, req.Tags)
	h.writeManageResult(c, err)
}

func (h *DesignsHandler) takedown(c *gin.Context) {
	h.setStatus(c, "takedown")
}

func (h *DesignsHandler) restore(c *gin.Context) {
	h.setStatus(c, "restore")
}

func (h *DesignsHandler) setStatus(c *gin.Context, action string) {
	_, err := h.hub.SetStatus(c.Request.Context(), auth.CurrentUser(c), c.Param("id"), action)
	h.writeManageResult(c, err)
}

func (h *DesignsHandler) delete(c *gin.Context) {
	err := h.hub.Delete(c.Request.Context(), auth.CurrentUser(c), c.Param("id"))
	switch {
	case errors.Is(err, hub.ErrNotFound):
		auth.Error(c, http.StatusNotFound, "not_found", "design not found")
	case errors.Is(err, hub.ErrForbidden):
		auth.Error(c, http.StatusForbidden, "forbidden", "owner or admin required")
	case err != nil:
		h.logger.WithError(err).Error("designs: delete failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to delete design")
	default:
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// writeManageResult maps hub errors and returns the updated detail.
func (h *DesignsHandler) writeManageResult(c *gin.Context, err error) {
	switch {
	case errors.Is(err, hub.ErrNotFound):
		auth.Error(c, http.StatusNotFound, "not_found", "design not found")
		return
	case errors.Is(err, hub.ErrForbidden):
		auth.Error(c, http.StatusForbidden, "forbidden", "owner or admin required")
		return
	case errors.Is(err, hub.ErrInvalidInput):
		auth.Error(c, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	case err != nil:
		auth.Error(c, http.StatusInternalServerError, "internal", "operation failed")
		return
	}
	d, err := h.hub.GetDetailByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load design")
		return
	}
	c.JSON(http.StatusOK, gin.H{"design": designDetailJSON(d)})
}

// hubDB exposes the hub's DB pool for transaction-free blob writes.
func (h *DesignsHandler) hubDB() store.DBTX {
	return h.hub.DB()
}

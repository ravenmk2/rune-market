package server

import (
	"context"
	"errors"
	"fmt"
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

// DesignsHandler serves the §8.3/§8.4 design endpoints, POST /images and
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

	apiAuth.POST("/images", h.uploadImage)
	apiAuth.POST("/designs/validate", h.validate)
	apiAuth.POST("/designs", h.publish)
	apiAuth.GET("/mine/designs", h.mine)
	apiAuth.PUT("/designs/:id", h.update)
	apiAuth.POST("/designs/:id/takedown", h.takedown)
	apiAuth.POST("/designs/:id/restore", h.restore)
	apiAuth.DELETE("/designs/:id", h.delete)
}

// --- response shaping (contract shapes) ---

// previewURL maps a blob sha to its /images path (nil when absent). The
// _640 thumbnail is always PNG; originals carry their stored extension,
// resolved via exts (sha256 → ext).
func previewURL(sha *string, suffix string, exts map[string]string) any {
	if sha == nil || *sha == "" {
		return nil
	}
	if suffix != "" {
		return "/images/" + *sha + suffix + ".png"
	}
	ext := exts[*sha]
	if ext == "" {
		return nil
	}
	return "/images/" + *sha + "." + ext
}

// previewExtMap batch-resolves the stored extensions of the given versions'
// preview images (one query, §8.3 URL building).
func previewExtMap(ctx context.Context, blobs *blob.Storage, db store.DBTX, versions ...*store.DesignmdVersion) (map[string]string, error) {
	var shas []string
	for _, v := range versions {
		if v == nil {
			continue
		}
		for _, sha := range []*string{v.PreviewDesktopSHA256, v.PreviewMobileSHA256} {
			if sha != nil && *sha != "" {
				shas = append(shas, *sha)
			}
		}
	}
	return blobs.ImageExts(ctx, db, shas)
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
		"preview_thumb_url": previewURL(it.PreviewDesktopSHA256, "_640", nil),
		"owner": gin.H{
			"username": it.OwnerUsername,
			"nickname": it.OwnerNickname,
		},
	}
}

func designVersionJSON(v *store.DesignmdVersion, exts map[string]string) gin.H {
	return gin.H{
		"version":             v.Version,
		"sha256":              v.SHA256,
		"preview_desktop_url": previewURL(v.PreviewDesktopSHA256, "", exts),
		"preview_mobile_url":  previewURL(v.PreviewMobileSHA256, "", exts),
		"created_at":          v.CreatedAt,
	}
}

func designDetailJSON(d *hub.DesignDetail, exts map[string]string) gin.H {
	out := designItemJSON(d.DesignItem)
	if d.Latest != nil {
		out["latest"] = designVersionJSON(d.Latest, exts)
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
	out, err := h.detailJSON(c.Request.Context(), d)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load design")
		return
	}
	c.JSON(http.StatusOK, gin.H{"design": out})
}

// detailJSON renders a detail with resolved preview image extensions.
func (h *DesignsHandler) detailJSON(ctx context.Context, d *hub.DesignDetail) (gin.H, error) {
	exts, err := previewExtMap(ctx, h.blobs, h.hub.DB(), d.Latest)
	if err != nil {
		return nil, err
	}
	return designDetailJSON(d, exts), nil
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
	exts, err := previewExtMap(c.Request.Context(), h.blobs, h.hub.DB(), versions...)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to list versions")
		return
	}
	out := make([]gin.H, 0, len(versions))
	for _, v := range versions {
		out = append(out, designVersionJSON(v, exts))
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
	exts, err := previewExtMap(c.Request.Context(), h.blobs, h.hub.DB(), v)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load version")
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": designVersionJSON(v, exts)})
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

var imageNameRe = regexp.MustCompile(`^[0-9a-f]{64}(_640)?\.(png|jpg)$`)

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
	p, err := h.blobs.Path(blob.KindImage, "x", "png")
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}

// --- upload endpoints ---

// uploadImage handles POST /images: raw body = PNG/JPG image, stored in its
// original format with content addressing (§8.4, §11). Used for DESIGN.md
// previews and skill icons.
func (h *DesignsHandler) uploadImage(c *gin.Context) {
	sum, ext, size, err := h.blobs.PutImage(c.Request.Context(), h.hubDB(), c.Request.Body)
	if err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_image", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"sha256": sum, "ext": ext, "size": size,
		"url":       "/images/" + sum + "." + ext,
		"thumb_url": "/images/" + sum + "_640.png",
	})
}

type validateDesignRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// validate handles POST /designs/validate (§8.4): JSON dry run returning the
// weak-validation report for the publish page precheck.
func (h *DesignsHandler) validate(c *gin.Context) {
	var req validateDesignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if !validateContentLength(c, req.Content) {
		return
	}
	c.JSON(http.StatusOK, designmd.Validate([]byte(req.Content)))
}

// validateContentLength enforces the 1MB content cap (§10.1).
func validateContentLength(c *gin.Context, content string) bool {
	if len(content) > designmd.MaxContentBytes {
		auth.Error(c, http.StatusRequestEntityTooLarge, "invalid_upload", "content exceeds 1MB limit")
		return false
	}
	return true
}

type publishDesignRequest struct {
	Content        string   `json:"content"`
	Name           string   `json:"name"`
	Summary        string   `json:"summary"`
	Version        string   `json:"version"`
	Tags           []string `json:"tags"`
	PreviewDesktop string   `json:"preview_desktop"`
	PreviewMobile  string   `json:"preview_mobile"`
}

// publish handles POST /designs (§8.4): JSON body; preview_* reference image
// blobs from POST /images.
func (h *DesignsHandler) publish(c *gin.Context) {
	var req publishDesignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if req.Name == "" {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "name is required")
		return
	}
	if req.Version == "" {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "version is required")
		return
	}
	for _, sha := range []string{req.PreviewDesktop, req.PreviewMobile} {
		if sha != "" && !sha256Re.MatchString(sha) {
			auth.Error(c, http.StatusBadRequest, "invalid_argument", "preview must be an image blob sha256 from POST /images")
			return
		}
	}
	if !validateContentLength(c, req.Content) {
		return
	}
	if report := designmd.Validate([]byte(req.Content)); report.HasErrors() {
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
		Name:           req.Name,
		Summary:        req.Summary,
		Version:        req.Version,
		Tags:           req.Tags,
		Content:        []byte(req.Content),
		PreviewDesktop: req.PreviewDesktop,
		PreviewMobile:  req.PreviewMobile,
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
	out, err := h.detailJSON(ctx, det)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load design")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"design": out})
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
	out, err := h.detailJSON(c.Request.Context(), d)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load design")
		return
	}
	c.JSON(http.StatusOK, gin.H{"design": out})
}

// hubDB exposes the hub's DB pool for transaction-free blob writes.
func (h *DesignsHandler) hubDB() store.DBTX {
	return h.hub.DB()
}

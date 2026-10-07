package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/hub"
	"github.com/ravenmk2/rune-market/internal/skillpkg"
	"github.com/ravenmk2/rune-market/internal/store"
)

// SkillsHandler serves the §8.3/§8.4 skill endpoints.
type SkillsHandler struct {
	apiSettings
	hub    *hub.Skills
	blobs  *blob.Storage
	logger *logrus.Logger
}

func NewSkillsHandler(h *hub.Skills, settings *store.SettingStore, blobs *blob.Storage, logger *logrus.Logger) *SkillsHandler {
	return &SkillsHandler{apiSettings: apiSettings{settings: settings}, hub: h, blobs: blobs, logger: logger}
}

// RegisterRoutes mounts public routes on api and authenticated routes on
// apiAuth (which must carry RequireAuth).
func (h *SkillsHandler) RegisterRoutes(api, apiAuth *gin.RouterGroup) {
	api.GET("/skills", h.list)
	api.GET("/skills/:ns/:name", h.detail)
	api.GET("/skills/:ns/:name/versions", h.versions)
	api.GET("/skills/:ns/:name/versions/:ver", h.version)
	api.GET("/skills/:ns/:name/versions/:ver/files", h.files)
	api.GET("/skills/:ns/:name/versions/:ver/file", h.file)
	api.GET("/skills/:ns/:name/versions/:ver/download", h.download)

	apiAuth.POST("/skills/validate", h.validate)
	apiAuth.POST("/skills", h.publish)
	apiAuth.GET("/mine/skills", h.mine)
	apiAuth.PUT("/skills/:id", h.update)
	apiAuth.POST("/skills/:id/takedown", h.takedown)
	apiAuth.POST("/skills/:id/restore", h.restore)
	apiAuth.DELETE("/skills/:id", h.delete)
}

// --- response shaping (contract shapes) ---

func skillItemJSON(it hub.SkillItem) gin.H {
	tags := it.Tags
	if tags == nil {
		tags = []string{}
	}
	return gin.H{
		"id":             it.Skill.ID,
		"namespace":      it.OwnerUsername,
		"name":           it.Skill.Name,
		"summary":        it.Skill.Summary,
		"official":       it.Skill.Official,
		"status":         it.Skill.Status,
		"tags":           tags,
		"latest_version": it.LatestVersion,
		"download_count": it.Skill.DownloadCount,
		"updated_at":     it.Skill.UpdatedAt,
		"owner": gin.H{
			"username": it.OwnerUsername,
			"nickname": it.OwnerNickname,
		},
	}
}

func versionJSON(v *store.SkillVersion) gin.H {
	return gin.H{
		"version":       v.Version,
		"description":   v.Description,
		"license":       v.License,
		"compatibility": v.Compatibility,
		"author":        v.Author,
		"harnesses":     json.RawMessage(v.Harnesses),
		"permissions":   json.RawMessage(v.Permissions),
		"file_count":    v.FileCount,
		"sha256":        v.SHA256,
		"size":          v.Size,
		"filename":      v.Filename,
		"created_at":    v.CreatedAt,
	}
}

func skillDetailJSON(d *hub.SkillDetail) gin.H {
	out := skillItemJSON(d.SkillItem)
	if d.Latest != nil {
		out["latest"] = versionJSON(d.Latest)
	}
	return out
}

// --- public read endpoints ---

func (h *SkillsHandler) list(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	ctx := c.Request.Context()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	items, total, err := h.hub.List(ctx, hub.ListFilter{
		Query:      c.Query("q"),
		Tag:        c.Query("tag"),
		Official:   c.Query("official") == "true" || c.Query("official") == "1",
		Sort:       c.Query("sort"),
		Page:       page,
		PageSize:   h.getInt(ctx, "page_size", 20),
		PublicOnly: true,
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
		"items": out, "total": total,
		"page": page, "page_size": h.getInt(ctx, "page_size", 20),
	})
}

// visibleDetail loads a detail and enforces that non-published skills are
// only visible to their owner or an admin (404 to others).
func (h *SkillsHandler) visibleDetail(c *gin.Context, ns, name string) *hub.SkillDetail {
	d, err := h.hub.GetDetail(c.Request.Context(), ns, name)
	if errors.Is(err, hub.ErrNotFound) {
		auth.Error(c, http.StatusNotFound, "not_found", "skill not found")
		return nil
	}
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load skill")
		return nil
	}
	if d.Skill.Status != store.SkillStatusPublished {
		u := auth.CurrentUser(c)
		if u == nil || (u.ID != d.Skill.OwnerID && u.Role != store.RoleAdmin) {
			auth.Error(c, http.StatusNotFound, "not_found", "skill not found")
			return nil
		}
	}
	return d
}

func (h *SkillsHandler) detail(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	d := h.visibleDetail(c, c.Param("ns"), c.Param("name"))
	if d == nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{"skill": skillDetailJSON(d)})
}

func (h *SkillsHandler) versions(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	d := h.visibleDetail(c, c.Param("ns"), c.Param("name"))
	if d == nil {
		return
	}
	versions, err := h.hub.ListVersions(c.Request.Context(), d.Skill.ID)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to list versions")
		return
	}
	out := make([]gin.H, 0, len(versions))
	for _, v := range versions {
		out = append(out, versionJSON(v))
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (h *SkillsHandler) version(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	v := h.visibleVersion(c)
	if v == nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{"version": versionJSON(v)})
}

// visibleVersion resolves :ns/:name/:ver with visibility checks.
func (h *SkillsHandler) visibleVersion(c *gin.Context) *store.SkillVersion {
	d := h.visibleDetail(c, c.Param("ns"), c.Param("name"))
	if d == nil {
		return nil
	}
	v, err := h.hub.GetVersion(c.Request.Context(), d.Skill.ID, c.Param("ver"))
	if errors.Is(err, hub.ErrNotFound) {
		auth.Error(c, http.StatusNotFound, "not_found", "version not found")
		return nil
	}
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load version")
		return nil
	}
	return v
}

func (h *SkillsHandler) files(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	v := h.visibleVersion(c)
	if v == nil {
		return
	}
	f, err := h.blobs.Open(blob.KindArchive, v.SHA256)
	if errors.Is(err, os.ErrNotExist) {
		auth.Error(c, http.StatusNotFound, "not_found", "archive blob missing")
		return
	}
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to open archive")
		return
	}
	defer func() { _ = f.Close() }()

	tree, err := skillpkg.Tree(f.Name())
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to read archive")
		return
	}
	if tree == nil {
		tree = []skillpkg.FileEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"files": tree})
}

func (h *SkillsHandler) file(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	v := h.visibleVersion(c)
	if v == nil {
		return
	}
	name := c.Query("path")
	if name == "" {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "path is required")
		return
	}
	f, err := h.blobs.Open(blob.KindArchive, v.SHA256)
	if err != nil {
		auth.Error(c, http.StatusNotFound, "not_found", "archive blob missing")
		return
	}
	defer func() { _ = f.Close() }()

	const maxPreview = 1 << 20 // §13: text preview capped at 1MB
	content, size, err := skillpkg.ReadFile(f.Name(), name, maxPreview)
	if errors.Is(err, os.ErrNotExist) {
		auth.Error(c, http.StatusNotFound, "not_found", "file not found in archive")
		return
	}
	if err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}
	resp := gin.H{"path": name, "size": size}
	if skillpkg.IsTextPath(name) {
		resp["content_type"] = "text"
		if content != nil {
			resp["content"] = string(content)
		}
	} else {
		resp["content_type"] = "binary"
	}
	c.JSON(http.StatusOK, resp)
}

func (h *SkillsHandler) download(c *gin.Context) {
	if !h.requireBrowseAuth(c) {
		return
	}
	if !h.getBool(c.Request.Context(), "anonymous_download", true) &&
		auth.CurrentUser(c) == nil {
		auth.Error(c, http.StatusUnauthorized, "unauthenticated", "login required")
		return
	}
	d := h.visibleDetail(c, c.Param("ns"), c.Param("name"))
	if d == nil {
		return
	}
	v, err := h.hub.GetVersion(c.Request.Context(), d.Skill.ID, c.Param("ver"))
	if err != nil {
		auth.Error(c, http.StatusNotFound, "not_found", "version not found")
		return
	}
	f, err := h.blobs.Open(blob.KindArchive, v.SHA256)
	if err != nil {
		auth.Error(c, http.StatusNotFound, "not_found", "archive blob missing")
		return
	}
	defer func() { _ = f.Close() }()

	h.hub.IncrDownload(c.Request.Context(), d.Skill.ID)
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, v.Filename))
	c.Header("Content-Type", "application/octet-stream")
	http.ServeContent(c.Writer, c.Request, v.Filename, v.CreatedAt, f)
}

// --- upload endpoints (raw body, §8 upload convention) ---

// saveUpload streams the raw body to a temp file, enforcing upload_max_mb,
// and returns its path, size and sha256. Caller must call cleanup.
func (h *SkillsHandler) saveUpload(c *gin.Context) (path string, size int64, sum string, cleanup func(), err error) {
	maxMB := h.getInt(c.Request.Context(), "upload_max_mb", 20)
	limit := int64(maxMB) << 20

	tmp, err := os.CreateTemp("", "runemarket-upload-*")
	if err != nil {
		return "", 0, "", nil, err
	}
	cleanup = func() { _ = os.Remove(tmp.Name()) }

	hash := sha256.New()
	size, err = io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(c.Request.Body, limit+1))
	if err != nil {
		_ = tmp.Close()
		cleanup()
		return "", 0, "", nil, err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", 0, "", nil, err
	}
	if size > limit {
		cleanup()
		return "", 0, "", nil, fmt.Errorf("upload exceeds %dMB limit", maxMB)
	}
	if size == 0 {
		cleanup()
		return "", 0, "", nil, errors.New("empty request body")
	}
	return tmp.Name(), size, hex.EncodeToString(hash.Sum(nil)), cleanup, nil
}

func (h *SkillsHandler) validate(c *gin.Context) {
	path, size, sum, cleanup, err := h.saveUpload(c)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "exceeds") {
			status = http.StatusRequestEntityTooLarge
		}
		auth.Error(c, status, "invalid_upload", err.Error())
		return
	}
	defer cleanup()

	pkg, err := skillpkg.Inspect(path, size, sum)
	if err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_upload", err.Error())
		return
	}
	c.JSON(http.StatusOK, pkg.Report)
}

func (h *SkillsHandler) publish(c *gin.Context) {
	version := c.Query("version")
	if version == "" {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", "version query param is required")
		return
	}
	tags := parseTags(c.Query("tags"))

	path, size, sum, cleanup, err := h.saveUpload(c)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "exceeds") {
			status = http.StatusRequestEntityTooLarge
		}
		auth.Error(c, status, "invalid_upload", err.Error())
		return
	}
	defer cleanup()

	pkg, err := skillpkg.Inspect(path, size, sum)
	if err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_upload", err.Error())
		return
	}
	if pkg.Report.HasErrors() {
		var details []string
		for _, chk := range pkg.Report.Checks {
			if chk.Level == skillpkg.LevelError {
				details = append(details, chk.Title+": "+chk.Detail)
			}
		}
		auth.ErrorDetails(c, http.StatusBadRequest, "invalid_package",
			"package validation failed", details)
		return
	}

	ctx := c.Request.Context()
	sk, _, err := h.hub.Publish(ctx, hub.PublishInput{
		Owner:          auth.CurrentUser(c),
		Version:        version,
		Tags:           tags,
		ArchivePath:    path,
		Package:        pkg,
		ReviewRequired: h.getStr(ctx, "artifact_review", "none") == "required",
	})
	switch {
	case errors.Is(err, hub.ErrVersionExists):
		auth.Error(c, http.StatusConflict, "version_exists", "version already published")
		return
	case errors.Is(err, hub.ErrInvalidInput):
		auth.Error(c, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	case err != nil:
		h.logger.WithError(err).Error("skills: publish failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to publish")
		return
	}

	d, err := h.hub.GetDetail(ctx, auth.CurrentUser(c).Username, sk.Name)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load skill")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"skill": skillDetailJSON(d)})
}

func parseTags(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range strings.Split(raw, ",") {
		t = strings.TrimSpace(t)
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// --- owner endpoints ---

func (h *SkillsHandler) mine(c *gin.Context) {
	ctx := c.Request.Context()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize := h.getInt(ctx, "page_size", 20)
	items, total, err := h.hub.List(ctx, hub.ListFilter{
		OwnerID:  auth.CurrentUser(c).ID,
		Page:     page,
		PageSize: pageSize,
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

type updateSkillRequest struct {
	Tags []string `json:"tags"`
}

func (h *SkillsHandler) update(c *gin.Context) {
	var req updateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	err := h.hub.UpdateTags(c.Request.Context(), auth.CurrentUser(c), c.Param("id"), req.Tags)
	h.writeManageResult(c, err)
}

func (h *SkillsHandler) takedown(c *gin.Context) {
	h.setStatus(c, "takedown")
}

func (h *SkillsHandler) restore(c *gin.Context) {
	h.setStatus(c, "restore")
}

func (h *SkillsHandler) setStatus(c *gin.Context, action string) {
	_, err := h.hub.SetStatus(c.Request.Context(), auth.CurrentUser(c), c.Param("id"), action)
	h.writeManageResult(c, err)
}

func (h *SkillsHandler) delete(c *gin.Context) {
	err := h.hub.Delete(c.Request.Context(), auth.CurrentUser(c), c.Param("id"))
	switch {
	case errors.Is(err, hub.ErrNotFound):
		auth.Error(c, http.StatusNotFound, "not_found", "skill not found")
	case errors.Is(err, hub.ErrForbidden):
		auth.Error(c, http.StatusForbidden, "forbidden", "owner or admin required")
	case err != nil:
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to delete skill")
	default:
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// writeManageResult maps hub errors and returns the updated detail.
func (h *SkillsHandler) writeManageResult(c *gin.Context, err error) {
	switch {
	case errors.Is(err, hub.ErrNotFound):
		auth.Error(c, http.StatusNotFound, "not_found", "skill not found")
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
	sk, err := h.hub.GetDetailByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to load skill")
		return
	}
	c.JSON(http.StatusOK, gin.H{"skill": skillDetailJSON(sk)})
}

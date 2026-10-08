// Package server assembles the Gin engine for setup and normal modes,
// security headers, the SPA handler and the in-process engine switcher.
package server

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/ravenmk2/rune-market/internal/auth"
)

type Deps struct {
	DataDir string
	Version string
	Logger  *logrus.Logger
	Static  fs.FS // embedded frontend, rooted at dist/
}

// NewNormalEngine builds the engine for normal mode (config.toml exists).
// Setup routes are absent, so they 404 per design §5.
func NewNormalEngine(deps Deps, authSvc *auth.Service, skillsH *SkillsHandler, designsH *DesignsHandler, adminH *AdminHandler, siteH *SiteHandler, accountH *AccountHandler) *gin.Engine {
	r := newBaseEngine(deps)

	api := r.Group("/api/v1", auth.CSRFProtect(), authSvc.Resolve())
	authSvc.RegisterRoutes(api.Group("/auth"))
	apiAuth := api.Group("", authSvc.RequireAuth())
	if skillsH != nil {
		skillsH.RegisterRoutes(api, apiAuth)
	}
	if designsH != nil {
		designsH.RegisterRoutes(r, api, apiAuth)
	}
	if adminH != nil {
		adminH.RegisterRoutes(api.Group("/admin", authSvc.RequireAdmin()))
	}
	if siteH != nil {
		siteH.RegisterRoutes(api)
	}
	if accountH != nil {
		accountH.RegisterRoutes(r, api, apiAuth)
	}

	r.NoRoute(spaHandler(deps.Static, func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			auth.Error(c, http.StatusNotFound, "not_found", "unknown API endpoint")
			return
		}
		serveIndex(c, deps.Static)
	}))
	return r
}

// NewSetupEngine builds the engine for setup mode: only the wizard API and
// the SPA shell; everything else redirects to /setup (design §5).
func NewSetupEngine(deps Deps, setupSvc *SetupService) *gin.Engine {
	r := newBaseEngine(deps)

	api := r.Group("/api/v1", auth.CSRFProtect())
	setupSvc.RegisterRoutes(api.Group("/setup"))

	r.NoRoute(spaHandler(deps.Static, func(c *gin.Context) {
		if c.Request.URL.Path == "/setup" {
			serveIndex(c, deps.Static)
			return
		}
		c.Redirect(http.StatusFound, "/setup")
	}))
	return r
}

func newBaseEngine(deps Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), requestLogger(deps.Logger), securityHeaders())
	return r
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

func requestLogger(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.WithFields(logrus.Fields{
			"method": c.Request.Method,
			"path":   c.Request.URL.Path,
			"status": c.Writer.Status(),
			"ms":     time.Since(start).Milliseconds(),
		}).Debug("request")
	}
}

// spaHandler serves static files when they exist, otherwise falls back
// (SPA index.html in normal mode, redirect to /setup in setup mode).
func spaHandler(static fs.FS, fallback gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			name := strings.TrimPrefix(path.Clean("/"+c.Request.URL.Path), "/")
			if name != "" && name != "." && serveFile(c, static, name) {
				return
			}
		}
		fallback(c)
	}
}

func serveFile(c *gin.Context, static fs.FS, name string) bool {
	f, err := static.Open(name)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		return false
	}
	rs, ok := f.(io.ReadSeeker)
	if !ok {
		return false
	}
	if strings.HasPrefix(name, "assets/") {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeContent(c.Writer, c.Request, name, time.Time{}, rs)
	return true
}

func serveIndex(c *gin.Context, static fs.FS) {
	c.Header("Cache-Control", "no-cache")
	if !serveFile(c, static, "index.html") {
		auth.Error(c, http.StatusNotFound, "frontend_missing",
			"frontend build not found (web/dist is a placeholder)")
	}
}

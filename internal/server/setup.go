package server

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/config"
	"github.com/ravenmk2/rune-market/internal/secret"
	"github.com/ravenmk2/rune-market/internal/store"
)

// SetupCompleteFunc builds and swaps in the normal-mode engine once the
// wizard finishes (in-process hot switch, design §5).
type SetupCompleteFunc func(ctx context.Context, db *sql.DB, cfg *config.Config) error

// Setup steps reported by GET /api/v1/setup/status.
const (
	SetupStepDatabase = "database"
	SetupStepAdmin    = "admin"
)

// Default settings written when the founder account is created (§6.3 keys).
var defaultSettings = map[string]string{
	"site_name":          "RuneMarket",
	"site_description":   "",
	"page_size":          "20",
	"registration_mode":  "open",
	"artifact_review":    "none",
	"upload_max_mb":      "20",
	"anonymous_browse":   "true",
	"anonymous_download": "true",
}

type SetupService struct {
	dataDir    string
	logger     *logrus.Logger
	onComplete SetupCompleteFunc

	mu   sync.Mutex
	step string
	db   *sql.DB
	cfg  *config.Config
}

func NewSetupService(dataDir string, logger *logrus.Logger, onComplete SetupCompleteFunc) *SetupService {
	return &SetupService{
		dataDir:    dataDir,
		logger:     logger,
		onComplete: onComplete,
		step:       SetupStepDatabase,
	}
}

func (s *SetupService) RegisterRoutes(g *gin.RouterGroup) {
	g.GET("/status", s.status)
	g.POST("/database/test", s.testDatabase)
	g.POST("/database", s.saveDatabase)
	g.POST("/admin", s.createAdmin)
}

func (s *SetupService) status(c *gin.Context) {
	s.mu.Lock()
	step := s.step
	s.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"mode": "setup", "step": step})
}

// databaseRequest mirrors the wizard's structured form (frontend contract).
type databaseRequest struct {
	Driver   string `json:"driver"`   // sqlite | mysql
	DataDir  string `json:"data_dir"` // sqlite: empty = server data dir
	Host     string `json:"host"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r *databaseRequest) config(defaultDataDir string) (*config.Config, error) {
	cfg := &config.Config{Database: config.Database{
		Driver:   r.Driver,
		DataDir:  r.DataDir,
		Host:     r.Host,
		Database: r.Database,
		Username: r.Username,
		Password: r.Password,
	}}
	// resolve the sqlite directory now so restarts do not depend on flags
	if cfg.Database.Driver == config.DriverSQLite && cfg.Database.DataDir == "" {
		cfg.Database.DataDir = defaultDataDir
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// testDatabase only verifies a MySQL connection; SQLite needs no check.
func (s *SetupService) testDatabase(c *gin.Context) {
	var req databaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if req.Driver == config.DriverSQLite {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	cfg, err := req.config(s.dataDir)
	if err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}
	db, err := store.Open(c.Request.Context(), cfg)
	if err != nil {
		auth.Error(c, http.StatusBadRequest, "db_connect_failed", err.Error())
		return
	}
	_ = db.Close()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// saveDatabase writes config.toml and initializes the schema (design §8.1).
func (s *SetupService) saveDatabase(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.step != SetupStepDatabase {
		auth.Error(c, http.StatusConflict, "invalid_step", "database is already configured")
		return
	}
	var req databaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	cfg, err := req.config(s.dataDir)
	if err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_argument", err.Error())
		return
	}
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to create data directory")
		return
	}

	ctx := c.Request.Context()
	db, err := store.Open(ctx, cfg)
	if err != nil {
		auth.Error(c, http.StatusBadRequest, "db_connect_failed", err.Error())
		return
	}
	if err := store.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		_ = db.Close()
		s.logger.WithError(err).Error("setup: migrate failed")
		auth.Error(c, http.StatusInternalServerError, "db_migrate_failed", "failed to initialize schema")
		return
	}
	if err := config.Save(s.dataDir, cfg); err != nil {
		_ = db.Close()
		s.logger.WithError(err).Error("setup: save config failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to write config.toml")
		return
	}
	s.db = db
	s.cfg = cfg
	s.step = SetupStepAdmin
	c.JSON(http.StatusOK, gin.H{"ok": true, "step": s.step})
}

type adminRequest struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

// createAdmin creates the founder account, writes default settings and the
// master secret, then hot-switches to normal mode (design §5, §8.1).
func (s *SetupService) createAdmin(c *gin.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.step != SetupStepAdmin {
		auth.Error(c, http.StatusConflict, "invalid_step", "configure the database first")
		return
	}
	var req adminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		auth.Error(c, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if details := auth.ValidateCredentials(req.Username, req.Nickname, req.Password); len(details) > 0 {
		auth.ErrorDetails(c, http.StatusBadRequest, "invalid_argument", "validation failed", details)
		return
	}

	ctx := c.Request.Context()
	stores := store.NewStores(s.db, s.cfg.Database.Driver)

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to hash password")
		return
	}
	now := store.Now()
	founder := &store.User{
		ID:           store.NewID(),
		Username:     req.Username,
		Nickname:     req.Nickname,
		PasswordHash: hash,
		Role:         store.RoleAdmin,
		IsFounder:    true,
		Status:       store.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := stores.Users.Create(ctx, founder); err != nil {
		if store.IsUniqueViolation(err) {
			auth.Error(c, http.StatusConflict, "username_taken", "username is already taken")
			return
		}
		s.logger.WithError(err).Error("setup: create founder failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to create founder")
		return
	}
	for k, v := range defaultSettings {
		if err := stores.Settings.Set(ctx, k, v); err != nil {
			s.logger.WithError(err).Error("setup: write default settings failed")
			auth.Error(c, http.StatusInternalServerError, "internal", "failed to write settings")
			return
		}
	}
	if _, err := secret.LoadOrCreate(s.dataDir); err != nil {
		s.logger.WithError(err).Error("setup: generate secret failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "failed to generate secret")
		return
	}
	if err := s.onComplete(ctx, s.db, s.cfg); err != nil {
		s.logger.WithError(err).Error("setup: switch to normal mode failed")
		auth.Error(c, http.StatusInternalServerError, "internal", "installation finished but activation failed; restart the server")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "mode": "normal"})
}

// RuneMarket entrypoint: config → secret → db → migrate → http (design §5).
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/config"
	"github.com/ravenmk2/rune-market/internal/hub"
	"github.com/ravenmk2/rune-market/internal/secret"
	"github.com/ravenmk2/rune-market/internal/server"
	"github.com/ravenmk2/rune-market/internal/store"
	"github.com/ravenmk2/rune-market/web"
)

// version is injected via -ldflags "-X main.version=<ver>" (design §14).
var version = "dev"

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dataFlag := flag.String("data", "", "data directory (default: RUNEMARKET_DATA or ./data)")
	flag.Parse()

	dataDir := *dataFlag
	if dataDir == "" {
		dataDir = os.Getenv("RUNEMARKET_DATA")
	}
	if dataDir == "" {
		dataDir = "./data"
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	gin.SetMode(gin.ReleaseMode)

	deps := server.Deps{
		DataDir: dataDir,
		Version: version,
		Logger:  logger,
		Static:  web.DistFS(),
	}

	cfg, err := config.Load(dataDir)
	var root *server.Switcher
	switch {
	case errors.Is(err, config.ErrNotExist):
		logger.Info("config.toml not found, entering setup mode")
		var sw *server.Switcher
		onComplete := func(ctx context.Context, db *sql.DB, cfg *config.Config) error {
			engine, err := buildNormalEngine(ctx, deps, db, cfg)
			if err != nil {
				return err
			}
			sw.Swap(engine)
			return nil
		}
		setupSvc := server.NewSetupService(dataDir, logger, onComplete)
		sw = server.NewSwitcher(server.NewSetupEngine(deps, setupSvc))
		root = sw
	case err != nil:
		logger.WithError(err).Fatal("failed to load config")
	default:
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		engine, err := buildNormalEngine(ctx, deps, nil, cfg)
		cancel()
		if err != nil {
			logger.WithError(err).Fatal("failed to initialize")
		}
		root = server.NewSwitcher(engine)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           root,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	logger.WithFields(logrus.Fields{"addr": *addr, "version": version, "data": dataDir}).
		Info("runemarket listening")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.WithError(err).Fatal("http server failed")
	}
}

// buildNormalEngine wires secret → db → migrate → stores → auth → engine.
// db may be nil, in which case it is opened from cfg.
func buildNormalEngine(ctx context.Context, deps server.Deps, db *sql.DB, cfg *config.Config) (*gin.Engine, error) {
	if _, err := secret.LoadOrCreate(deps.DataDir); err != nil {
		return nil, err
	}
	var err error
	if db == nil {
		db, err = store.Open(ctx, cfg)
		if err != nil {
			return nil, err
		}
	}
	if err := store.Migrate(ctx, db, cfg.Database.Driver); err != nil {
		return nil, err
	}
	stores := store.NewStores(db, cfg.Database.Driver)
	authSvc := auth.NewService(stores.Users, stores.Sessions, stores.Settings)
	blobs := blob.New(deps.DataDir)
	skillsSvc := hub.NewSkills(db, cfg.Database.Driver, blobs)
	skillsH := server.NewSkillsHandler(skillsSvc, stores.Settings, blobs, deps.Logger)
	designsSvc := hub.NewDesigns(db, cfg.Database.Driver, blobs)
	designsH := server.NewDesignsHandler(designsSvc, stores.Settings, blobs, deps.Logger)
	return server.NewNormalEngine(deps, authSvc, skillsH, designsH), nil
}

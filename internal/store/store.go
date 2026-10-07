// Package store is the database layer: hand-written SQL over database/sql,
// dialect differences concentrated in migrations and a few SQL fragments.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ravenmk2/rune-market/internal/config"
)

// Dialect names, identical to config driver names and migration directories.
const (
	DialectSQLite = config.DriverSQLite
	DialectMySQL  = config.DriverMySQL
)

// ErrNotFound is returned by Get* methods when the row does not exist.
var ErrNotFound = errors.New("store: not found")

// NewID returns a UUIDv7 primary key: 32 hex chars, no dashes (design §6).
func NewID() string {
	return strings.ReplaceAll(uuid.Must(uuid.NewV7()).String(), "-", "")
}

// Now returns the current UTC time truncated to milliseconds (design §6).
func Now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

// DefaultDataDir is the data directory used when neither the -data flag,
// RUNEMARKET_DATA nor the wizard's data_dir overrides it (design §4).
const DefaultDataDir = "./data"

// sqliteDSN builds the §6.1 DSN for a database file path.
func sqliteDSN(file string) string {
	return "file:" + filepath.ToSlash(file) +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
}

// Open connects to the configured database and verifies the connection.
func Open(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	var db *sql.DB
	var err error
	switch cfg.Database.Driver {
	case DialectSQLite:
		dataDir := cfg.Database.DataDir
		if dataDir == "" {
			dataDir = DefaultDataDir
		}
		db, err = sql.Open(sqliteDriverName,
			sqliteDSN(filepath.Join(dataDir, "runemarket.db")))
	case DialectMySQL:
		db, err = sql.Open("mysql", BuildMySQLDSN(cfg.Database))
	default:
		return nil, fmt.Errorf("store: unsupported driver %q", cfg.Database.Driver)
	}
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", cfg.Database.Driver, err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping %s: %w", cfg.Database.Driver, err)
	}
	return db, nil
}

// IsUniqueViolation reports whether err is a unique-constraint violation
// from either supported driver.
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || // sqlite
		strings.Contains(msg, "Error 1062") // mysql duplicate entry
}

// Stores bundles the per-entity stores over one connection pool.
type Stores struct {
	DB            *sql.DB
	Dialect       string
	Users         *UserStore
	Sessions      *SessionStore
	Settings      *SettingStore
	Skills        *SkillStore
	SkillVersions *SkillVersionStore
	Tags          *TagStore
}

func NewStores(db *sql.DB, dialect string) *Stores {
	return &Stores{
		DB:            db,
		Dialect:       dialect,
		Users:         NewUserStore(db),
		Sessions:      NewSessionStore(db),
		Settings:      NewSettingStore(db, dialect),
		Skills:        NewSkillStore(db),
		SkillVersions: NewSkillVersionStore(db),
		Tags:          NewTagStore(db, dialect),
	}
}

// Package config reads and writes ./data/config.toml and decides whether
// the process runs in setup mode (no config file yet) or normal mode.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Driver names supported by the store layer.
const (
	DriverSQLite = "sqlite"
	DriverMySQL  = "mysql"
)

// ErrNotExist marks a missing config.toml, i.e. setup mode.
var ErrNotExist = errors.New("config: config.toml does not exist")

// Database holds the structured connection settings written by the setup
// wizard; the store layer builds driver DSNs from these fields.
type Database struct {
	Driver string `toml:"driver"` // sqlite | mysql

	// sqlite: directory of runemarket.db, empty means ./data
	DataDir string `toml:"data_dir,omitempty"`

	// mysql
	Host     string `toml:"host,omitempty"`
	Database string `toml:"database,omitempty"`
	Username string `toml:"username,omitempty"`
	Password string `toml:"password,omitempty"`
}

type Config struct {
	Database Database `toml:"database"`
}

func path(dataDir string) string {
	return filepath.Join(dataDir, "config.toml")
}

// Load reads config.toml from dataDir. Returns ErrNotExist in setup mode.
func Load(dataDir string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path(dataDir), &cfg); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotExist
		}
		return nil, fmt.Errorf("config: load: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Validate() error {
	switch c.Database.Driver {
	case DriverSQLite:
		// everything optional; empty data_dir means the default ./data
		return nil
	case DriverMySQL:
		if c.Database.Host == "" || c.Database.Database == "" || c.Database.Username == "" {
			return errors.New("config: mysql requires host, database and username")
		}
		return nil
	default:
		return fmt.Errorf("config: unsupported database driver %q", c.Database.Driver)
	}
}

// Save writes config.toml with 0600 permissions (it may hold a DB password).
func Save(dataDir string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("config: create data dir: %w", err)
	}
	f, err := os.OpenFile(path(dataDir), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("config: write: %w", err)
	}
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		_ = f.Close()
		return fmt.Errorf("config: encode: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("config: write: %w", err)
	}
	return nil
}

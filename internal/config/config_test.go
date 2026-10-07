package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	if _, err := Load(t.TempDir()); !errors.Is(err, ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{Database: Database{
		Driver:   DriverMySQL,
		Host:     "db.example.com:3306",
		Database: "runemarket",
		Username: "rune",
		Password: "s3cret",
	}}
	if err := Save(dir, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 0600: only owner bits may be set (Windows ignores Unix file modes)
	if runtime.GOOS != "windows" {
		st, err := os.Stat(filepath.Join(dir, "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if perm := st.Mode().Perm(); perm&0o077 != 0 {
			t.Fatalf("config.toml permissions too open: %o", perm)
		}
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Database != cfg.Database {
		t.Fatalf("round-trip mismatch: %+v", got.Database)
	}
}

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"sqlite default", Config{Database: Database{Driver: DriverSQLite}}, false},
		{"sqlite with dir", Config{Database: Database{Driver: DriverSQLite, DataDir: "/var/lib/rm"}}, false},
		{"mysql full", Config{Database: Database{
			Driver: DriverMySQL, Host: "h:3306", Database: "db", Username: "u",
		}}, false},
		{"mysql missing host", Config{Database: Database{
			Driver: DriverMySQL, Database: "db", Username: "u",
		}}, true},
		{"mysql missing database", Config{Database: Database{
			Driver: DriverMySQL, Host: "h", Username: "u",
		}}, true},
		{"mysql missing username", Config{Database: Database{
			Driver: DriverMySQL, Host: "h", Database: "db",
		}}, true},
		{"unknown driver", Config{Database: Database{Driver: "postgres"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

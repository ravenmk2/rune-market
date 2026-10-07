package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"

	"github.com/ravenmk2/rune-market/internal/config"
)

// openTestDB opens a migrated database for the given dialect.
// MySQL tests require RUNEMARKET_TEST_MYSQL_DSN (any parseable DSN, e.g.
// "root:pass@tcp(127.0.0.1:3306)/runemarket_test") and are skipped in -short mode.
func openTestDB(t *testing.T, dialect string) *sql.DB {
	t.Helper()
	ctx := context.Background()

	cfg := &config.Config{}
	switch dialect {
	case DialectSQLite:
		cfg.Database.Driver = DialectSQLite
		cfg.Database.DataDir = t.TempDir()
	case DialectMySQL:
		if testing.Short() {
			t.Skip("mysql test skipped in -short mode")
		}
		dsn := os.Getenv("RUNEMARKET_TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("RUNEMARKET_TEST_MYSQL_DSN not set")
		}
		parsed, err := mysql.ParseDSN(dsn)
		if err != nil {
			t.Fatalf("parse RUNEMARKET_TEST_MYSQL_DSN: %v", err)
		}
		cfg.Database.Driver = DialectMySQL
		cfg.Database.Host = parsed.Addr
		cfg.Database.Database = parsed.DBName
		cfg.Database.Username = parsed.User
		cfg.Database.Password = parsed.Passwd
	default:
		t.Fatalf("unknown dialect %q", dialect)
	}

	db, err := Open(ctx, cfg)
	if err != nil {
		t.Fatalf("open %s: %v", dialect, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if dialect == DialectMySQL {
		// isolate from other runs against a shared dev database
		for _, table := range []string{"session", "setting", "user", "blob", "schema_migrations"} {
			if _, err := db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
				t.Fatalf("drop %s: %v", table, err)
			}
		}
	}
	if err := Migrate(ctx, db, dialect); err != nil {
		t.Fatalf("migrate %s: %v", dialect, err)
	}
	return db
}

func dialects() []string {
	return []string{DialectSQLite, DialectMySQL}
}

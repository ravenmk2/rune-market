package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/sqlite migrations/mysql
var migrationsFS embed.FS

// Migration files live in migrations/<dialect>/NNNN_name.sql, applied in
// filename order. Limitations of the statement splitter: SQL files must not
// contain ";" or "--" inside string literals (DDL only).

var schemaMigrationsDDL = map[string]string{
	DialectSQLite: `CREATE TABLE IF NOT EXISTS schema_migrations (
  version    INTEGER PRIMARY KEY,
  applied_at TIMESTAMP NOT NULL
)`,
	DialectMySQL: `CREATE TABLE IF NOT EXISTS schema_migrations (
  version    INT PRIMARY KEY,
  applied_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin`,
}

// Migrate applies all pending migrations for the dialect.
func Migrate(ctx context.Context, db *sql.DB, dialect string) error {
	ddl, ok := schemaMigrationsDDL[dialect]
	if !ok {
		return fmt.Errorf("store: no migrations for dialect %q", dialect)
	}
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("store: create schema_migrations: %w", err)
	}

	applied := map[int]bool{}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("store: read schema_migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return err
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations/"+dialect)
	if err != nil {
		return fmt.Errorf("store: read migrations dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		version, err := migrationVersion(name)
		if err != nil {
			return err
		}
		if applied[version] {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + dialect + "/" + name)
		if err != nil {
			return err
		}
		if err := applyMigration(ctx, db, version, string(body)); err != nil {
			return fmt.Errorf("store: migration %s: %w", name, err)
		}
	}
	return nil
}

func migrationVersion(name string) (int, error) {
	base, _, found := strings.Cut(name, "_")
	if !found {
		return 0, fmt.Errorf("store: bad migration filename %q", name)
	}
	return strconv.Atoi(base)
}

func applyMigration(ctx context.Context, db *sql.DB, version int, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range splitStatements(body) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		version, Now()); err != nil {
		return err
	}
	return tx.Commit()
}

// splitStatements strips "--" comments and splits on ";" at statement level.
func splitStatements(body string) []string {
	var noComments strings.Builder
	for line := range strings.Lines(body) {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i] + "\n"
		}
		noComments.WriteString(line)
	}
	var out []string
	for _, stmt := range strings.Split(noComments.String(), ";") {
		if stmt = strings.TrimSpace(stmt); stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}

package store

import (
	"context"
	"strings"
	"testing"

	"github.com/ravenmk2/rune-market/internal/config"
)

func TestDesignPreviewMigrationPreservesLegacy(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, &config.Config{Database: config.Database{Driver: DialectSQLite, DataDir: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, schemaMigrationsDDL[DialectSQLite]); err != nil {
		t.Fatal(err)
	}
	entries, err := migrationsFS.ReadDir("migrations/sqlite")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		version, err := migrationVersion(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if version >= 6 {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/sqlite/" + entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if err := applyMigration(ctx, db, version, string(body)); err != nil {
			t.Fatal(err)
		}
	}
	now := Now()
	u := &User{ID: NewID(), Username: "legacy", Nickname: "Legacy", PasswordHash: "hash", Role: RoleUser, Status: StatusActive, CreatedAt: now, UpdatedAt: now}
	if err := NewUserStore(db).Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	d := &Designmd{ID: NewID(), OwnerID: u.ID, Name: "legacy", Status: DesignStatusPublished, CreatedAt: now, UpdatedAt: now}
	if err := NewDesignmdStore(db).Create(ctx, d); err != nil {
		t.Fatal(err)
	}
	sha, vid := strings.Repeat("a", 64), NewID()
	if _, err := db.ExecContext(ctx, "INSERT INTO blob (sha256, kind, size, ext, ref_count, created_at) VALUES (?, 'image', 100, 'png', 3, ?)", sha, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO designmd_version (id, designmd_id, version, content, sha256, preview_desktop_sha256, preview_mobile_sha256, created_at) VALUES (?, ?, '1.0.0', '# Legacy', ?, ?, ?, ?)`, vid, d.ID, sha, sha, sha, now); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Migrate(ctx, db, DialectSQLite); err != nil {
			t.Fatal(err)
		}
	}
	v, err := NewDesignmdVersionStore(db).GetByID(ctx, vid)
	if err != nil {
		t.Fatal(err)
	}
	if v.Content != "# Legacy" || len(v.Previews()) != 2 || v.Previews()[0].Theme != "default" || *v.PreviewDesktopSHA256 != sha || *v.PreviewMobileSHA256 != sha {
		t.Fatalf("legacy changed: %+v", v)
	}
	var refs int
	if err := db.QueryRowContext(ctx, "SELECT ref_count FROM blob WHERE sha256 = ?", sha).Scan(&refs); err != nil || refs != 3 {
		t.Fatalf("refs=%d err=%v", refs, err)
	}
}

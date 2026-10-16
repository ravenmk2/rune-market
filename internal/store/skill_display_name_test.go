package store

import (
	"context"
	"testing"

	"github.com/ravenmk2/rune-market/internal/config"
)

func TestSkillDisplayNameMigration(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, &config.Config{Database: config.Database{
		Driver: DialectSQLite, DataDir: t.TempDir(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, schemaMigrationsDDL[DialectSQLite]); err != nil {
		t.Fatal(err)
	}
	// Build the pre-display-name schema and insert a real legacy skill.
	entries, err := migrationsFS.ReadDir("migrations/sqlite")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		version, err := migrationVersion(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if version >= 5 {
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
	u := &User{ID: NewID(), Username: "raven", Nickname: "Raven", PasswordHash: "hash",
		Role: RoleUser, Status: StatusActive, CreatedAt: now, UpdatedAt: now}
	if err := NewUserStore(db).Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	id := NewID()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO skill (id, owner_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id, u.ID, "skill-creator", now, now); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db, DialectSQLite); err != nil {
		t.Fatal(err)
	}
	sk, err := NewSkillStore(db).GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if sk.Name != "skill-creator" || sk.DisplayName != sk.Name {
		t.Fatalf("legacy skill after migration: %+v", sk)
	}
	// Re-running migrations must not overwrite an edited display name.
	if err := NewSkillStore(db).UpdateDisplayName(ctx, id, "技能创建者", Now()); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db, DialectSQLite); err != nil {
		t.Fatal(err)
	}
	sk, err = NewSkillStore(db).GetByID(ctx, id)
	if err != nil || sk.DisplayName != "技能创建者" {
		t.Fatalf("repeat migration: skill=%+v err=%v", sk, err)
	}
}

package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMigrateIdempotent(t *testing.T) {
	for _, dialect := range dialects() {
		t.Run(dialect, func(t *testing.T) {
			db := openTestDB(t, dialect)
			ctx := context.Background()
			if err := Migrate(ctx, db, dialect); err != nil {
				t.Fatalf("second migrate: %v", err)
			}
			var n int
			if err := db.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Fatalf("expected 1 applied migration, got %d", n)
			}
		})
	}
}

func TestSplitStatements(t *testing.T) {
	body := `
-- a comment
CREATE TABLE a (id CHAR(32) PRIMARY KEY); -- inline comment
CREATE TABLE b (
  id CHAR(32) PRIMARY KEY
);
`
	stmts := splitStatements(body)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d: %v", len(stmts), stmts)
	}
	if strings.Contains(stmts[0], "--") {
		t.Fatalf("comment not stripped: %q", stmts[0])
	}
}

func TestUserStoreCRUD(t *testing.T) {
	for _, dialect := range dialects() {
		t.Run(dialect, func(t *testing.T) {
			db := openTestDB(t, dialect)
			users := NewUserStore(db)
			ctx := context.Background()

			now := Now()
			u := &User{
				ID: NewID(), Username: "alice-" + NewID()[:8], Nickname: "Alice",
				PasswordHash: "hash", Role: RoleAdmin, IsFounder: true,
				Status: StatusActive, CreatedAt: now, UpdatedAt: now,
			}
			if err := users.Create(ctx, u); err != nil {
				t.Fatalf("create: %v", err)
			}
			if len(u.ID) != 32 {
				t.Fatalf("id should be 32 hex chars, got %q", u.ID)
			}

			got, err := users.GetByUsername(ctx, u.Username)
			if err != nil {
				t.Fatalf("get by username: %v", err)
			}
			if got.ID != u.ID || got.Nickname != "Alice" || got.Role != RoleAdmin ||
				!got.IsFounder || got.Status != StatusActive || got.Bio != "" {
				t.Fatalf("round-trip mismatch: %+v", got)
			}
			if !got.CreatedAt.Equal(now) {
				t.Fatalf("created_at mismatch: want %v got %v", now, got.CreatedAt)
			}

			got, err = users.GetByID(ctx, u.ID)
			if err != nil || got.Username != u.Username {
				t.Fatalf("get by id: %v %+v", err, got)
			}

			if _, err := users.GetByID(ctx, NewID()); !errors.Is(err, ErrNotFound) {
				t.Fatalf("expected ErrNotFound, got %v", err)
			}

			dup := &User{
				ID: NewID(), Username: u.Username, Nickname: "Dup",
				PasswordHash: "hash", Role: RoleUser, Status: StatusActive,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := users.Create(ctx, dup); !IsUniqueViolation(err) {
				t.Fatalf("expected unique violation, got %v", err)
			}
		})
	}
}

func TestSessionStoreCRUD(t *testing.T) {
	for _, dialect := range dialects() {
		t.Run(dialect, func(t *testing.T) {
			db := openTestDB(t, dialect)
			users := NewUserStore(db)
			sessions := NewSessionStore(db)
			ctx := context.Background()

			now := Now()
			u := &User{
				ID: NewID(), Username: "bob-" + NewID()[:8], Nickname: "Bob",
				PasswordHash: "hash", Role: RoleUser, Status: StatusActive,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := users.Create(ctx, u); err != nil {
				t.Fatal(err)
			}

			exp := now.Add(14 * 24 * time.Hour)
			sess := &Session{
				ID: NewID(), UserID: u.ID, TokenHash: strings.Repeat("ab", 32),
				IP: "127.0.0.1", UserAgent: "test", ExpiresAt: exp, CreatedAt: now,
			}
			if err := sessions.Create(ctx, sess); err != nil {
				t.Fatalf("create: %v", err)
			}

			got, err := sessions.GetByTokenHash(ctx, sess.TokenHash)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if got.UserID != u.ID || got.IP != "127.0.0.1" || got.UserAgent != "test" {
				t.Fatalf("round-trip mismatch: %+v", got)
			}
			if !got.ExpiresAt.Equal(exp) {
				t.Fatalf("expires_at mismatch: want %v got %v", exp, got.ExpiresAt)
			}

			newExp := exp.Add(24 * time.Hour)
			if err := sessions.Touch(ctx, sess.ID, newExp); err != nil {
				t.Fatalf("touch: %v", err)
			}
			got, _ = sessions.GetByTokenHash(ctx, sess.TokenHash)
			if !got.ExpiresAt.Equal(newExp) {
				t.Fatalf("touch not applied: %v", got.ExpiresAt)
			}

			if err := sessions.DeleteByTokenHash(ctx, sess.TokenHash); err != nil {
				t.Fatalf("delete: %v", err)
			}
			if _, err := sessions.GetByTokenHash(ctx, sess.TokenHash); !errors.Is(err, ErrNotFound) {
				t.Fatalf("expected ErrNotFound, got %v", err)
			}

			old := &Session{
				ID: NewID(), UserID: u.ID, TokenHash: strings.Repeat("cd", 32),
				ExpiresAt: now.Add(-time.Hour), CreatedAt: now.Add(-48 * time.Hour),
			}
			if err := sessions.Create(ctx, old); err != nil {
				t.Fatal(err)
			}
			n, err := sessions.DeleteExpired(ctx)
			if err != nil || n != 1 {
				t.Fatalf("delete expired: n=%d err=%v", n, err)
			}

			// FK cascade: deleting the user removes their sessions
			live := &Session{
				ID: NewID(), UserID: u.ID, TokenHash: strings.Repeat("ef", 32),
				ExpiresAt: exp, CreatedAt: now,
			}
			if err := sessions.Create(ctx, live); err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `DELETE FROM user WHERE id = ?`, u.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := sessions.GetByTokenHash(ctx, live.TokenHash); !errors.Is(err, ErrNotFound) {
				t.Fatalf("expected cascade delete, got %v", err)
			}
		})
	}
}

func TestSettingStoreCRUD(t *testing.T) {
	for _, dialect := range dialects() {
		t.Run(dialect, func(t *testing.T) {
			db := openTestDB(t, dialect)
			settings := NewSettingStore(db, dialect)
			ctx := context.Background()

			if _, err := settings.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("expected ErrNotFound, got %v", err)
			}
			if err := settings.Set(ctx, "site_name", "RuneMarket"); err != nil {
				t.Fatalf("set: %v", err)
			}
			if v, err := settings.Get(ctx, "site_name"); err != nil || v != "RuneMarket" {
				t.Fatalf("get: %q %v", v, err)
			}
			// upsert on same key
			if err := settings.Set(ctx, "site_name", "RM"); err != nil {
				t.Fatalf("update: %v", err)
			}
			if v, _ := settings.Get(ctx, "site_name"); v != "RM" {
				t.Fatalf("upsert failed: %q", v)
			}

			if err := settings.Set(ctx, "page_size", "20"); err != nil {
				t.Fatal(err)
			}
			all, err := settings.GetAll(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(all) != 2 || all["site_name"] != "RM" || all["page_size"] != "20" {
				t.Fatalf("get all: %v", all)
			}
		})
	}
}

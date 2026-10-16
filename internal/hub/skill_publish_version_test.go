package hub

import (
	"context"
	"errors"
	"testing"

	"github.com/ravenmk2/rune-market/internal/store"
)

func TestPublishBoundSkillVersion(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	archive := env.archiveSHA(t, "skill-creator", "package description")
	icon := env.iconSHA(t)
	sk, _, err := env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.2.3", ArchiveSHA256: archive,
		DisplayName: "技能创建者", Tags: []string{"tools"}, IconSHA256: icon,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The legacy endpoint may have uploaded an older version most recently.
	// Bound publication must still compare against the semantic maximum.
	env.publish(t, "skill-creator", "older upload", "1.0.0", []string{"tools"})
	other := &store.User{ID: store.NewID(), Username: "admin", Nickname: "Admin",
		PasswordHash: "x", Role: store.RoleAdmin, Status: store.StatusActive,
		CreatedAt: store.Now(), UpdatedAt: store.Now()}
	if err := env.db.Users.Create(ctx, other); err != nil {
		t.Fatal(err)
	}
	wrongArchive := env.archiveSHA(t, "different-skill", "different package")
	newIcon := env.iconSHA(t)
	for _, tc := range []struct {
		name, target, archive, version string
		actor                          *store.User
		want                           error
	}{
		{"lower than maximum", sk.ID, archive, "1.1.0", env.owner, ErrInvalidInput},
		{"duplicate", sk.ID, archive, "1.2.3", env.owner, ErrVersionExists},
		{"equal precedence", sk.ID, archive, "1.2.3+build.9", env.owner, ErrInvalidInput},
		{"package mismatch", sk.ID, wrongArchive, "1.2.4", env.owner, ErrInvalidInput},
		{"missing target", store.NewID(), archive, "1.2.4", env.owner, ErrNotFound},
		{"admin is not owner", sk.ID, archive, "1.2.4", other, ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			archiveRefs, iconRefs := env.blobRefs(t, tc.archive), env.blobRefs(t, newIcon)
			_, _, err := env.skills.Publish(ctx, PublishInput{
				Owner: tc.actor, SkillID: tc.target, ArchiveSHA256: tc.archive, Version: tc.version,
				DisplayName: "must not persist", Tags: []string{"changed"}, IconSHA256: newIcon,
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if env.blobRefs(t, tc.archive) != archiveRefs || env.blobRefs(t, newIcon) != iconRefs {
				t.Fatal("rejected publish changed blob references")
			}
		})
	}
	before, err := env.skills.GetDetailByID(ctx, sk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Skill.DisplayName != "技能创建者" || *before.Skill.IconSHA256 != icon || before.LatestVersion != "1.0.0" {
		t.Fatalf("rejected requests changed skill: %+v", before.Skill)
	}
	// Omitted metadata preserves display name, tags and icon; description is
	// taken from the new package. Prereleases are supported by the server.
	for _, version := range []string{"1.2.4-beta.1", "1.2.4"} {
		got, _, err := env.skills.Publish(ctx, PublishInput{
			Owner: env.owner, SkillID: sk.ID, ArchiveSHA256: archive, Version: version,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != sk.ID || got.DisplayName != "技能创建者" || got.IconSHA256 == nil || *got.IconSHA256 != icon {
			t.Fatalf("metadata not inherited: %+v", got)
		}
	}
	detail, err := env.skills.GetDetailByID(ctx, sk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Tags) != 1 || detail.Tags[0] != "tools" || detail.Latest.Description != "package description" {
		t.Fatalf("inherited tags=%v, description=%q", detail.Tags, detail.Latest.Description)
	}
	_, _, err = env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, SkillID: sk.ID, ArchiveSHA256: archive, Version: "1.2.5", Tags: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	detail, err = env.skills.GetDetailByID(ctx, sk.ID)
	if err != nil || len(detail.Tags) != 0 {
		t.Fatalf("explicit empty tags: detail=%+v err=%v", detail, err)
	}
	versions, err := env.skills.ListVersions(ctx, sk.ID)
	if err != nil || len(versions) != 5 {
		t.Fatalf("history: len=%d err=%v", len(versions), err)
	}
	var count int
	if err := env.db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM skill`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("unexpected new skill: count=%d err=%v", count, err)
	}
}

package hub

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/store"
)

const sampleDesignDoc = `# Acme Design

## Overview

Brand system.

## Colors

Primary #1a73e8.
`

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (e *testEnv) uploadImage(t *testing.T, w, h int) string {
	t.Helper()
	sum, _, _, err := e.blobs.PutImage(context.Background(), e.db.DB, bytes.NewReader(makePNG(t, w, h)))
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func (e *testEnv) publishDesign(t *testing.T, name, summary, version string, tags []string, preview string) (*store.Designmd, *store.DesignmdVersion) {
	t.Helper()
	d, v, err := e.designs.Publish(context.Background(), DesignPublishInput{
		Owner: e.owner, Name: name, Summary: summary, Version: version,
		Tags: tags, Content: []byte(sampleDesignDoc), PreviewDesktop: preview,
	})
	if err != nil {
		t.Fatalf("publish design %s@%s: %v", name, version, err)
	}
	return d, v
}

func TestDesignPublishAndVersions(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	preview := env.uploadImage(t, 800, 600)

	d, v1 := env.publishDesign(t, "acme-ui", "Acme design system", "1.0.0", []string{"品牌"}, preview)
	if d.Status != store.DesignStatusPublished {
		t.Fatalf("status: %q", d.Status)
	}
	if v1.SHA256 == "" || v1.PreviewDesktopSHA256 == nil || *v1.PreviewDesktopSHA256 != preview {
		t.Fatalf("version: %+v", v1)
	}

	// preview blob ref_count: 1 (upload) + 1 (publish ref) = 2
	var refs int
	if err := env.db.DB.QueryRowContext(ctx,
		"SELECT ref_count FROM `blob` WHERE sha256 = ?", preview).Scan(&refs); err != nil {
		t.Fatal(err)
	}
	if refs != 2 {
		t.Fatalf("preview refs: %d", refs)
	}

	// second version bumps ref again and moves latest
	env.publishDesign(t, "acme-ui", "Acme v2", "1.1.0", []string{"品牌", "深色"}, preview)
	det, err := env.designs.GetDetail(ctx, "raven", "acme-ui")
	if err != nil {
		t.Fatal(err)
	}
	if det.LatestVersion != "1.1.0" || det.Design.Summary != "Acme v2" {
		t.Fatalf("latest: %+v", det.LatestVersion)
	}
	if len(det.Tags) != 2 {
		t.Fatalf("tags: %v", det.Tags)
	}
	if det.PreviewDesktopSHA256 == nil || *det.PreviewDesktopSHA256 != preview {
		t.Fatalf("preview sha: %+v", det.PreviewDesktopSHA256)
	}
	_ = env.db.DB.QueryRowContext(ctx,
		"SELECT ref_count FROM `blob` WHERE sha256 = ?", preview).Scan(&refs)
	if refs != 3 {
		t.Fatalf("preview refs after v2: %d", refs)
	}

	// version conflict
	_, _, err = env.designs.Publish(ctx, DesignPublishInput{
		Owner: env.owner, Name: "acme-ui", Version: "1.1.0",
		Content: []byte(sampleDesignDoc),
	})
	if !errors.Is(err, ErrVersionExists) {
		t.Fatalf("expected conflict, got %v", err)
	}

	// versions semver-descending
	versions, err := env.designs.ListVersions(ctx, d.ID)
	if err != nil || len(versions) != 2 || versions[0].Version != "1.1.0" {
		t.Fatalf("versions: %v %v", versions, err)
	}
}

func TestDesignPublishValidation(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	// bad name
	_, _, err := env.designs.Publish(ctx, DesignPublishInput{
		Owner: env.owner, Name: "Bad_Name", Version: "1.0.0", Content: []byte("# x"),
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("bad name: %v", err)
	}
	// unknown preview blob
	_, _, err = env.designs.Publish(ctx, DesignPublishInput{
		Owner: env.owner, Name: "ok-name", Version: "1.0.0", Content: []byte("# x"),
		PreviewDesktop: "0000000000000000000000000000000000000000000000000000000000000000",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown preview: %v", err)
	}
	// review required → pending
	d, _, err := env.designs.Publish(ctx, DesignPublishInput{
		Owner: env.owner, Name: "pending-one", Version: "1.0.0",
		Content: []byte("# x"), ReviewRequired: true,
	})
	if err != nil || d.Status != store.DesignStatusPending {
		t.Fatalf("review: %v %q", err, d.Status)
	}
}

func TestDesignUpdateStatusDelete(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	preview := env.uploadImage(t, 800, 600)
	d, _ := env.publishDesign(t, "gone", "s", "1.0.0", []string{"a"}, preview)

	other := &store.User{
		ID: store.NewID(), Username: "other", Nickname: "O",
		PasswordHash: "x", Role: store.RoleUser, Status: store.StatusActive,
		CreatedAt: store.Now(), UpdatedAt: store.Now(),
	}
	if err := env.db.Users.Create(ctx, other); err != nil {
		t.Fatal(err)
	}

	// non-owner cannot manage
	if err := env.designs.Update(ctx, other, d.ID, strptr("x"), nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	// owner edits summary + tags
	summary := "new summary"
	if err := env.designs.Update(ctx, env.owner, d.ID, &summary, []string{"b", "c"}); err != nil {
		t.Fatal(err)
	}
	det, _ := env.designs.GetDetailByID(ctx, d.ID)
	if det.Design.Summary != "new summary" || len(det.Tags) != 2 || det.Tags[0] != "b" {
		t.Fatalf("after update: %+v tags=%v", det.Design.Summary, det.Tags)
	}
	// tags-only update leaves summary
	if err := env.designs.Update(ctx, env.owner, d.ID, nil, []string{"d"}); err != nil {
		t.Fatal(err)
	}
	det, _ = env.designs.GetDetailByID(ctx, d.ID)
	if det.Design.Summary != "new summary" || len(det.Tags) != 1 || det.Tags[0] != "d" {
		t.Fatalf("after tags-only update: %+v %v", det.Design.Summary, det.Tags)
	}

	// takedown/restore
	if _, err := env.designs.SetStatus(ctx, env.owner, d.ID, "takedown"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.designs.SetStatus(ctx, env.owner, d.ID, "restore"); err != nil {
		t.Fatal(err)
	}

	// delete releases the preview ref and removes files
	previewPath, _ := env.blobs.Path(blob.KindImage, preview, "png")
	if err := env.designs.Delete(ctx, env.owner, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.designs.GetDetailByID(ctx, d.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	// publish held 2 refs (upload+publish), delete releases 1 → row remains with ref 1
	var refs int
	if err := env.db.DB.QueryRowContext(ctx,
		"SELECT ref_count FROM `blob` WHERE sha256 = ?", preview).Scan(&refs); err != nil {
		t.Fatal(err)
	}
	if refs != 1 {
		t.Fatalf("preview refs after delete: %d", refs)
	}
	if _, err := os.Stat(previewPath); err != nil {
		t.Fatalf("preview should still exist (ref 1): %v", err)
	}
}

func strptr(s string) *string { return &s }

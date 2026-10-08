package hub

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/config"
	"github.com/ravenmk2/rune-market/internal/store"
)

type testEnv struct {
	db      *store.Stores
	skills  *Skills
	designs *Designs
	blobs   *blob.Storage
	dataDir string
	owner   *store.User
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctx := context.Background()
	dataDir := t.TempDir()
	cfg := &config.Config{Database: config.Database{
		Driver: store.DialectSQLite, DataDir: t.TempDir(),
	}}
	db, err := store.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(ctx, db, store.DialectSQLite); err != nil {
		t.Fatal(err)
	}
	stores := store.NewStores(db, store.DialectSQLite)

	now := store.Now()
	owner := &store.User{
		ID: store.NewID(), Username: "raven", Nickname: "Raven",
		PasswordHash: "x", Role: store.RoleUser, Status: store.StatusActive,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := stores.Users.Create(ctx, owner); err != nil {
		t.Fatal(err)
	}
	blobs := blob.New(dataDir)
	return &testEnv{
		db: stores, skills: NewSkills(db, store.DialectSQLite, blobs),
		designs: NewDesigns(db, store.DialectSQLite, blobs),
		blobs:   blobs, dataDir: dataDir, owner: owner,
	}
}

// buildPackage creates a real zip on disk and returns its path.
func buildPackage(t *testing.T, name, description string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "skill.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	files := map[string]string{
		name + "/SKILL.md": "---\nname: " + name + "\ndescription: " + description +
			"\nlicense: MIT\nallowed-tools: Bash(python3:*) Read\n---\n\n# Body\n",
		name + "/extra.txt": "hello",
	}
	for n, body := range files {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

// archiveSHA uploads a built package as an archive blob (the POST /archives
// step of the staged publish flow) and returns its sha256.
func (e *testEnv) archiveSHA(t *testing.T, name, description string) string {
	t.Helper()
	f, err := os.Open(buildPackage(t, name, description))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	sum, _, err := e.blobs.Put(context.Background(), e.db.DB, blob.KindArchive, f)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

// iconSHA uploads an icon image blob (the POST /images step); each call
// varies the pixel content so icons never dedup to the same sha.
func (e *testEnv) iconSHA(t *testing.T) string {
	t.Helper()
	iconSeq++
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for i := range img.Pix {
		img.Pix[i] = uint8(iconSeq + i)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	sum, _, _, err := e.blobs.PutImage(context.Background(), e.db.DB, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

var iconSeq int

func (e *testEnv) blobRefs(t *testing.T, sha string) int {
	t.Helper()
	var refs int
	if err := e.db.DB.QueryRowContext(context.Background(),
		"SELECT ref_count FROM `blob` WHERE sha256 = ?", sha).Scan(&refs); err != nil {
		t.Fatal(err)
	}
	return refs
}

func (e *testEnv) publish(t *testing.T, name, description, version string, tags []string) (*store.Skill, *store.SkillVersion) {
	t.Helper()
	sk, v, err := e.skills.Publish(context.Background(), PublishInput{
		Owner: e.owner, Version: version, Tags: tags,
		ArchiveSHA256: e.archiveSHA(t, name, description),
	})
	if err != nil {
		t.Fatalf("publish %s@%s: %v", name, version, err)
	}
	return sk, v
}

func TestPublishAndVersions(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	sk, v1 := env.publish(t, "pdf", "PDF tools v1", "1.0.0", []string{"文档", "pdf"})
	if sk.Status != store.SkillStatusPublished {
		t.Fatalf("status: %q", sk.Status)
	}
	if v1.Author != "Raven" { // metadata.author fallback to nickname
		t.Fatalf("author fallback: %q", v1.Author)
	}
	if v1.Filename != "pdf-1.0.0.zip" || v1.FileCount != 2 {
		t.Fatalf("version: %+v", v1)
	}

	// new version moves the latest pointer and refreshes the summary
	env.publish(t, "pdf", "PDF tools v2", "1.1.0", []string{"文档"})
	d, err := env.skills.GetDetail(ctx, "raven", "pdf")
	if err != nil {
		t.Fatal(err)
	}
	if d.LatestVersion != "1.1.0" || d.Skill.Summary != "PDF tools v2" {
		t.Fatalf("latest: %+v", d.LatestVersion)
	}
	if len(d.Tags) != 1 || d.Tags[0] != "文档" {
		t.Fatalf("tags replaced: %v", d.Tags)
	}

	// version conflict
	_, _, err = env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.1.0", ArchiveSHA256: env.archiveSHA(t, "pdf", "d"),
	})
	if !errors.Is(err, ErrVersionExists) {
		t.Fatalf("expected version conflict, got %v", err)
	}

	// versions come back semver-descending
	env.publish(t, "pdf", "PDF tools v10", "1.10.0", nil)
	versions, err := env.skills.ListVersions(ctx, sk.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{versions[0].Version, versions[1].Version, versions[2].Version}
	if got[0] != "1.10.0" || got[1] != "1.1.0" || got[2] != "1.0.0" {
		t.Fatalf("version order: %v", got)
	}

	// blob dedup: identical archives share one blob with ref_count per version
	var refs int
	if err := env.db.DB.QueryRowContext(ctx,
		`SELECT ref_count FROM blob WHERE sha256 = ?`, v1.SHA256).Scan(&refs); err != nil {
		t.Fatal(err)
	}
	if refs < 1 {
		t.Fatalf("ref_count: %d", refs)
	}
}

// TestPublishStagedRejects covers the staged publish guards: the archive
// reference must point at an existing archive blob.
func TestPublishStagedRejects(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	// unknown sha
	_, _, err := env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.0.0",
		ArchiveSHA256: "0000000000000000000000000000000000000000000000000000000000000000",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown archive: %v", err)
	}

	// an image blob is not an archive
	imgSHA := env.iconSHA(t)
	_, _, err = env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.0.0", ArchiveSHA256: imgSHA,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("image as archive: %v", err)
	}

	// icon must be an existing image blob
	_, _, err = env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.0.0",
		ArchiveSHA256: env.archiveSHA(t, "bad-icon", "d"),
		IconSHA256:    "1111111111111111111111111111111111111111111111111111111111111111",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown icon: %v", err)
	}
	archiveSHA := env.archiveSHA(t, "bad-icon2", "d")
	_, _, err = env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.0.0", ArchiveSHA256: archiveSHA, IconSHA256: archiveSHA,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("archive as icon: %v", err)
	}
	// failed publishes must not leave the archive binding ref behind
	if got := env.blobRefs(t, archiveSHA); got != 1 {
		t.Fatalf("archive refs after failed publish: %d want 1 (upload only)", got)
	}
}

func TestPublishIcon(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	icon := env.iconSHA(t)

	// publish with icon: skill row references it, icon gains a binding ref
	sk, _, err := env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.0.0",
		ArchiveSHA256: env.archiveSHA(t, "iconed", "d"), IconSHA256: icon,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sk.IconSHA256 == nil || *sk.IconSHA256 != icon {
		t.Fatalf("icon: %+v", sk.IconSHA256)
	}
	if got := env.blobRefs(t, icon); got != 2 {
		t.Fatalf("icon refs: %d want 2 (upload+bind)", got)
	}
	// detail carries the resolved extension for URL building
	d, err := env.skills.GetDetail(ctx, "raven", "iconed")
	if err != nil {
		t.Fatal(err)
	}
	if d.IconExt != "png" {
		t.Fatalf("icon ext: %q", d.IconExt)
	}

	// republishing a new version with a different icon swaps the reference
	icon2 := env.iconSHA(t)
	_, _, err = env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.1.0",
		ArchiveSHA256: env.archiveSHA(t, "iconed", "d2"), IconSHA256: icon2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := env.blobRefs(t, icon); got != 1 {
		t.Fatalf("old icon refs after swap: %d want 1", got)
	}
	if got := env.blobRefs(t, icon2); got != 2 {
		t.Fatalf("new icon refs after swap: %d want 2", got)
	}
	d, _ = env.skills.GetDetail(ctx, "raven", "iconed")
	if d.Skill.IconSHA256 == nil || *d.Skill.IconSHA256 != icon2 {
		t.Fatalf("icon after swap: %+v", d.Skill.IconSHA256)
	}

	// republishing without icon keeps the current one
	env.publish(t, "iconed", "d3", "1.2.0", nil)
	d, _ = env.skills.GetDetail(ctx, "raven", "iconed")
	if d.Skill.IconSHA256 == nil || *d.Skill.IconSHA256 != icon2 {
		t.Fatalf("icon lost on plain republish: %+v", d.Skill.IconSHA256)
	}
}

func TestUpdateIcon(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	sk, _ := env.publish(t, "ico", "d", "1.0.0", nil)
	icon := env.iconSHA(t)

	// nil leaves the field unchanged
	if err := env.skills.Update(ctx, env.owner, sk.ID, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	d, _ := env.skills.GetDetailByID(ctx, sk.ID)
	if d.Skill.IconSHA256 != nil {
		t.Fatalf("icon should stay unset: %+v", d.Skill.IconSHA256)
	}

	// set
	if err := env.skills.Update(ctx, env.owner, sk.ID, nil, nil, nil, &icon); err != nil {
		t.Fatal(err)
	}
	if got := env.blobRefs(t, icon); got != 2 {
		t.Fatalf("icon refs after set: %d want 2", got)
	}

	// change: new gains a binding ref, old is released
	icon2 := env.iconSHA(t)
	if err := env.skills.Update(ctx, env.owner, sk.ID, nil, nil, nil, &icon2); err != nil {
		t.Fatal(err)
	}
	if got := env.blobRefs(t, icon); got != 1 {
		t.Fatalf("old icon refs after change: %d want 1", got)
	}
	if got := env.blobRefs(t, icon2); got != 2 {
		t.Fatalf("new icon refs after change: %d want 2", got)
	}

	// setting the same icon again is a no-op
	if err := env.skills.Update(ctx, env.owner, sk.ID, nil, nil, nil, &icon2); err != nil {
		t.Fatal(err)
	}
	if got := env.blobRefs(t, icon2); got != 2 {
		t.Fatalf("icon refs after no-op set: %d want 2", got)
	}

	// unknown / wrong-kind icon rejected
	bad := "2222222222222222222222222222222222222222222222222222222222222222"
	if err := env.skills.Update(ctx, env.owner, sk.ID, nil, nil, nil, &bad); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown icon: %v", err)
	}
	archiveSHA := env.archiveSHA(t, "not-icon", "d")
	if err := env.skills.Update(ctx, env.owner, sk.ID, nil, nil, nil, &archiveSHA); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("archive as icon: %v", err)
	}

	// clear releases the binding ref
	empty := ""
	if err := env.skills.Update(ctx, env.owner, sk.ID, nil, nil, nil, &empty); err != nil {
		t.Fatal(err)
	}
	if got := env.blobRefs(t, icon2); got != 1 {
		t.Fatalf("icon refs after clear: %d want 1 (upload only)", got)
	}
	d, _ = env.skills.GetDetailByID(ctx, sk.ID)
	if d.Skill.IconSHA256 != nil {
		t.Fatalf("icon should be cleared: %+v", d.Skill.IconSHA256)
	}
}

// TestDeleteReleasesIcon: skill deletion releases the icon binding ref too.
func TestDeleteReleasesIcon(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	icon := env.iconSHA(t)
	sk, _, err := env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.0.0",
		ArchiveSHA256: env.archiveSHA(t, "gone-icon", "d"), IconSHA256: icon,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.skills.Delete(ctx, env.owner, sk.ID); err != nil {
		t.Fatal(err)
	}
	if got := env.blobRefs(t, icon); got != 1 {
		t.Fatalf("icon refs after delete: %d want 1 (upload only)", got)
	}
}

func TestPublishReviewRequired(t *testing.T) {
	env := newTestEnv(t)
	sk, _, err := env.skills.Publish(context.Background(), PublishInput{
		Owner: env.owner, Version: "1.0.0", ArchiveSHA256: env.archiveSHA(t, "reviewed", "d"),
		ReviewRequired: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sk.Status != store.SkillStatusPending {
		t.Fatalf("status: %q want pending", sk.Status)
	}
}

func TestPublishBadVersion(t *testing.T) {
	env := newTestEnv(t)
	_, _, err := env.skills.Publish(context.Background(), PublishInput{
		Owner: env.owner, Version: "v1.0", ArchiveSHA256: env.archiveSHA(t, "x", "d"),
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestListFilters(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	env.publish(t, "alpha", "PDF extraction toolkit", "1.0.0", []string{"文档"})
	env.publish(t, "beta", "image resizing", "1.0.0", []string{"图片"})

	items, total, err := env.skills.List(ctx, ListFilter{PublicOnly: true})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("list: %v total=%d", err, total)
	}
	if items[0].OwnerUsername != "raven" || items[0].OwnerNickname != "Raven" {
		t.Fatalf("owner: %+v", items[0])
	}

	_, total, _ = env.skills.List(ctx, ListFilter{PublicOnly: true, Query: "pdf"})
	if total != 1 {
		t.Fatalf("q filter: %d", total)
	}
	_, total, _ = env.skills.List(ctx, ListFilter{PublicOnly: true, Tag: "图片"})
	if total != 1 {
		t.Fatalf("tag filter: %d", total)
	}
	_, total, _ = env.skills.List(ctx, ListFilter{PublicOnly: true, Official: true})
	if total != 0 {
		t.Fatalf("official filter: %d", total)
	}
}

func TestSetStatusAndPermissions(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	sk, _ := env.publish(t, "owned", "d", "1.0.0", nil)

	other := &store.User{
		ID: store.NewID(), Username: "other", Nickname: "O",
		PasswordHash: "x", Role: store.RoleUser, Status: store.StatusActive,
		CreatedAt: store.Now(), UpdatedAt: store.Now(),
	}
	if err := env.db.Users.Create(ctx, other); err != nil {
		t.Fatal(err)
	}

	// non-owner cannot manage
	if _, err := env.skills.SetStatus(ctx, other, sk.ID, "takedown"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if err := env.skills.Update(ctx, other, sk.ID, nil, nil, []string{"x"}, nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if err := env.skills.Delete(ctx, other, sk.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	// owner: takedown → restore
	sk, err := env.skills.SetStatus(ctx, env.owner, sk.ID, "takedown")
	if err != nil || sk.Status != store.SkillStatusTakenDown {
		t.Fatalf("takedown: %v %q", err, sk.Status)
	}
	sk, err = env.skills.SetStatus(ctx, env.owner, sk.ID, "restore")
	if err != nil || sk.Status != store.SkillStatusPublished {
		t.Fatalf("restore: %v %q", err, sk.Status)
	}

	// admin can manage others' skills
	admin := &store.User{
		ID: store.NewID(), Username: "boss", Nickname: "B",
		PasswordHash: "x", Role: store.RoleAdmin, Status: store.StatusActive,
		CreatedAt: store.Now(), UpdatedAt: store.Now(),
	}
	if err := env.db.Users.Create(ctx, admin); err != nil {
		t.Fatal(err)
	}
	if _, err := env.skills.SetStatus(ctx, admin, sk.ID, "takedown"); err != nil {
		t.Fatalf("admin takedown: %v", err)
	}
}

func TestPublishOfficialAndUpdateMeta(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	// Official=true on publish sets the flag at creation
	sk, _, err := env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.0.0", ArchiveSHA256: env.archiveSHA(t, "off", "original description"),
		Official: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sk.Official {
		t.Fatal("expected official skill")
	}

	// republish keeps the existing row's flag untouched
	env.publish(t, "off", "v2 description", "1.1.0", nil)
	d, err := env.skills.GetDetail(ctx, "raven", "off")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Skill.Official {
		t.Fatal("official flag lost after republish")
	}

	// summary + current-version description are editable
	sum, desc := "new summary", "new description"
	if err := env.skills.Update(ctx, env.owner, sk.ID, &sum, &desc, nil, nil); err != nil {
		t.Fatal(err)
	}
	d, err = env.skills.GetDetail(ctx, "raven", "off")
	if err != nil {
		t.Fatal(err)
	}
	if d.Skill.Summary != sum || d.Latest.Description != desc {
		t.Fatalf("summary=%q description=%q", d.Skill.Summary, d.Latest.Description)
	}
	// older version keeps its own description
	v1, err := env.skills.GetVersion(ctx, sk.ID, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if v1.Description != "original description" {
		t.Fatalf("v1 description changed: %q", v1.Description)
	}

	// nil fields leave everything unchanged; tags still replaceable
	if err := env.skills.Update(ctx, env.owner, sk.ID, nil, nil, []string{"文档"}, nil); err != nil {
		t.Fatal(err)
	}
	d, _ = env.skills.GetDetail(ctx, "raven", "off")
	if len(d.Tags) != 1 || d.Tags[0] != "文档" || d.Skill.Summary != sum {
		t.Fatalf("tags=%v summary=%q", d.Tags, d.Skill.Summary)
	}
}

func TestPublishDescriptionOverride(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	// form description replaces the package meta description for both the
	// skill summary and the published version
	sk, v, err := env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.0.0", ArchiveSHA256: env.archiveSHA(t, "ovr", "package description"),
		Description: "  表单说明覆盖  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sk.Summary != "表单说明覆盖" || v.Description != "表单说明覆盖" {
		t.Fatalf("override: summary=%q description=%q", sk.Summary, v.Description)
	}

	// empty / whitespace-only description keeps the package meta description
	sk, v, err = env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.0.0", ArchiveSHA256: env.archiveSHA(t, "keep", "package description"),
		Description: "   ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sk.Summary != "package description" || v.Description != "package description" {
		t.Fatalf("fallback: summary=%q description=%q", sk.Summary, v.Description)
	}

	// over-length override rejected (1024 rune limit, same as update)
	_, _, err = env.skills.Publish(ctx, PublishInput{
		Owner: env.owner, Version: "1.0.0", ArchiveSHA256: env.archiveSHA(t, "toolong", "d"),
		Description: strings.Repeat("长", 1025),
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestDeleteReleasesBlob(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	sk, v := env.publish(t, "gone", "d", "1.0.0", nil)

	blobPath, _ := env.blobs.Path(blob.KindArchive, v.SHA256, "")
	if _, err := os.Stat(blobPath); err != nil {
		t.Fatalf("blob should exist: %v", err)
	}
	// upload holds 1 ref, the version binding adds 1
	if got := env.blobRefs(t, v.SHA256); got != 2 {
		t.Fatalf("archive refs after publish: %d want 2 (upload+bind)", got)
	}

	if err := env.skills.Delete(ctx, env.owner, sk.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := env.skills.GetDetailByID(ctx, sk.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	// delete releases the binding ref; the upload ref keeps the blob alive
	if got := env.blobRefs(t, v.SHA256); got != 1 {
		t.Fatalf("archive refs after delete: %d want 1", got)
	}
	if _, err := os.Stat(blobPath); err != nil {
		t.Fatalf("blob should survive on the upload ref: %v", err)
	}
	var n int
	_ = env.db.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM skill_version WHERE skill_id = ?`, sk.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("versions should cascade, %d left", n)
	}

	// releasing the upload ref deletes the file
	if err := env.blobs.Release(ctx, env.db.DB, v.SHA256); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(blobPath); !os.IsNotExist(err) {
		t.Fatalf("blob file should be deleted: %v", err)
	}
}

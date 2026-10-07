package hub

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/config"
	"github.com/ravenmk2/rune-market/internal/skillpkg"
	"github.com/ravenmk2/rune-market/internal/store"
)

type testEnv struct {
	db      *store.Stores
	skills  *Skills
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
		blobs: blobs, dataDir: dataDir, owner: owner,
	}
}

// buildPackage creates a real zip and inspects it.
func buildPackage(t *testing.T, name, description string) (string, *skillpkg.Package) {
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
	st, _ := os.Stat(p)
	pkg, err := skillpkg.Inspect(p, st.Size(), "")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Report.HasErrors() {
		t.Fatalf("test package has errors: %+v", pkg.Report.Checks)
	}
	return p, pkg
}

func (e *testEnv) publish(t *testing.T, name, description, version string, tags []string) (*store.Skill, *store.SkillVersion) {
	t.Helper()
	path, pkg := buildPackage(t, name, description)
	sk, v, err := e.skills.Publish(context.Background(), PublishInput{
		Owner: e.owner, Version: version, Tags: tags,
		ArchivePath: path, Package: pkg,
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
		Owner: env.owner, Version: "1.1.0", ArchivePath: mustPkg(t, "pdf"),
		Package: mustInspect(t, "pdf"),
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

func mustPkg(t *testing.T, name string) string {
	t.Helper()
	p, _ := buildPackage(t, name, "d")
	return p
}

func mustInspect(t *testing.T, name string) *skillpkg.Package {
	t.Helper()
	_, pkg := buildPackage(t, name, "d")
	return pkg
}

func TestPublishReviewRequired(t *testing.T) {
	env := newTestEnv(t)
	path, pkg := buildPackage(t, "reviewed", "d")
	sk, _, err := env.skills.Publish(context.Background(), PublishInput{
		Owner: env.owner, Version: "1.0.0", ArchivePath: path, Package: pkg,
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
	path, pkg := buildPackage(t, "x", "d")
	_, _, err := env.skills.Publish(context.Background(), PublishInput{
		Owner: env.owner, Version: "v1.0", ArchivePath: path, Package: pkg,
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
	if err := env.skills.UpdateTags(ctx, other, sk.ID, []string{"x"}); !errors.Is(err, ErrForbidden) {
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

func TestDeleteReleasesBlob(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	sk, v := env.publish(t, "gone", "d", "1.0.0", nil)

	blobPath, _ := env.blobs.Path(blob.KindArchive, v.SHA256)
	if _, err := os.Stat(blobPath); err != nil {
		t.Fatalf("blob should exist: %v", err)
	}

	if err := env.skills.Delete(ctx, env.owner, sk.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := env.skills.GetDetailByID(ctx, sk.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if _, err := os.Stat(blobPath); !os.IsNotExist(err) {
		t.Fatalf("blob file should be deleted: %v", err)
	}
	var n int
	_ = env.db.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM skill_version WHERE skill_id = ?`, sk.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("versions should cascade, %d left", n)
	}
}

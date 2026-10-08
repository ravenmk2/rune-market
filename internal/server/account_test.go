package server

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"testing"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/hub"
	"github.com/ravenmk2/rune-market/internal/store"
)

// newAccountEnv wires all handlers including account, with avatars under
// the temp data dir.
func newAccountEnv(t *testing.T) (*skillsEnv, *blob.Storage, string) {
	env := newSkillsEnv(t)
	stores := env.stores
	authSvc := auth.NewService(stores.Users, stores.Sessions, stores.Settings)
	dataDir := t.TempDir()
	blobs := blob.New(dataDir)
	deps := testDeps(t)
	skillsSvc := hub.NewSkills(stores.DB, store.DialectSQLite, blobs)
	designsSvc := hub.NewDesigns(stores.DB, store.DialectSQLite, blobs)
	skillsH := NewSkillsHandler(skillsSvc, stores.Settings, blobs, deps.Logger)
	designsH := NewDesignsHandler(designsSvc, stores.Settings, blobs, deps.Logger)
	adminH := NewAdminHandler(stores, skillsSvc, designsSvc, blobs, deps.Logger,
		"test-version", deps.DataDir, store.DialectSQLite)
	accountH := NewAccountHandler(stores, skillsSvc, designsSvc, blobs, deps.Logger, dataDir)
	env.engine = NewNormalEngine(deps, authSvc, skillsH, designsH, adminH,
		NewSiteHandler(stores.Settings, "test-version"), accountH)
	return env, blobs, dataDir
}

var (
	colorRed  = color.RGBA{R: 220, A: 255}
	colorBlue = color.RGBA{B: 220, A: 255}
)

func widePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// left half red, right half blue so center-crop is observable
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				img.Set(x, y, colorRed)
			} else {
				img.Set(x, y, colorBlue)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAvatarLifecycle(t *testing.T) {
	env, blobs, _ := newAccountEnv(t)
	otherID := userID(t, env, "other")

	// upload a wide image: cropped to a centered square + 3 thumbnails
	w := env.doRaw(t, http.MethodPost, "/api/v1/account/avatar",
		widePNG(t, 400, 200), env.other)
	if w.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", w.Code, w.Body)
	}
	if decode(t, w)["user"].(map[string]any)["has_avatar"] != true {
		t.Fatalf("has_avatar: %s", w.Body)
	}

	// original is a square crop (200x200 out of 400x200)
	f, err := os.Open(blobs.AvatarPath(otherID, ""))
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(f)
	_ = f.Close()
	if err != nil || format != "png" || cfg.Width != 200 || cfg.Height != 200 {
		t.Fatalf("original: %v %s %dx%d", err, format, cfg.Width, cfg.Height)
	}

	// thumbnails exist at all whitelist sizes and are served with short cache
	for _, size := range []string{"32", "64", "128"} {
		w = env.do(t, http.MethodGet, "/avatars/"+otherID+"_"+size+".png", "", nil, nil)
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "public, max-age=60" {
			t.Fatalf("avatar %s: %d %v", size, w.Code, w.Header())
		}
		if w.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("avatar %s content type: %v", size, w.Header())
		}
	}
	w = env.do(t, http.MethodGet, "/avatars/"+otherID+".png", "", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("avatar original: %d", w.Code)
	}
	// bad names and missing avatars 404
	for _, p := range []string{
		"/avatars/" + otherID + "_999.png",
		"/avatars/nothex.png",
		"/avatars/00000000000000000000000000000000.png",
	} {
		w = env.do(t, http.MethodGet, p, "", nil, nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s: %d", p, w.Code)
		}
	}

	// overwrite replaces the group
	w = env.doRaw(t, http.MethodPost, "/api/v1/account/avatar",
		widePNG(t, 100, 100), env.other)
	if w.Code != http.StatusOK {
		t.Fatalf("overwrite: %d", w.Code)
	}

	// GIF and text rejected
	w = env.doRaw(t, http.MethodPost, "/api/v1/account/avatar",
		[]byte("GIF89a fake gif content"), env.other)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("gif: %d", w.Code)
	}

	// delete: files gone, has_avatar=false
	w = env.do(t, http.MethodDelete, "/api/v1/account/avatar", "", env.other, nil)
	if w.Code != http.StatusOK ||
		decode(t, w)["user"].(map[string]any)["has_avatar"] != false {
		t.Fatalf("delete: %d %s", w.Code, w.Body)
	}
	w = env.do(t, http.MethodGet, "/avatars/"+otherID+".png", "", nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("avatar after delete: %d", w.Code)
	}

	// anonymous cannot upload
	w = env.doRaw(t, http.MethodPost, "/api/v1/account/avatar",
		widePNG(t, 10, 10), nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous upload: %d", w.Code)
	}
}

func TestUserProfile(t *testing.T) {
	env, _, _ := newAccountEnv(t)
	ctx := context.Background()

	// raven publishes a skill and a design; one skill is taken down
	env.publish(t, "pub-skill", "d", "1.0.0", "工具")
	env.publishDesign(t, "pub-design", "s", "1.0.0", nil, "", "")
	down := env.publish(t, "down-skill", "d", "1.0.0", "")
	w := env.do(t, http.MethodPost, "/api/v1/skills/"+down["id"].(string)+"/takedown", "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("takedown: %d %s", w.Code, w.Body)
	}

	w = env.do(t, http.MethodGet, "/api/v1/users/raven", "", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("profile: %d %s", w.Code, w.Body)
	}
	m := decode(t, w)
	u := m["user"].(map[string]any)
	if u["username"] != "raven" || u["nickname"] != "Raven" {
		t.Fatalf("user: %v", u)
	}
	for _, f := range []string{"id", "username", "nickname", "bio", "has_avatar", "updated_at", "created_at"} {
		if _, ok := u[f]; !ok {
			t.Fatalf("profile user missing %q", f)
		}
	}
	if u["id"] == "" || u["updated_at"] == "" {
		t.Fatalf("id/updated_at should be populated: %v", u)
	}
	if _, leaked := u["role"]; leaked {
		t.Fatal("role should not leak in public profile")
	}
	skills := m["skills"].([]any)
	designs := m["designs"].([]any)
	if len(skills) != 1 || skills[0].(map[string]any)["name"] != "pub-skill" {
		t.Fatalf("skills (only published): %v", skills)
	}
	if len(designs) != 1 {
		t.Fatalf("designs: %v", designs)
	}

	// unknown user 404
	w = env.do(t, http.MethodGet, "/api/v1/users/nobody", "", nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown user: %d", w.Code)
	}

	// anonymous_browse off gates the profile
	if err := env.stores.Settings.Set(ctx, "anonymous_browse", "false"); err != nil {
		t.Fatal(err)
	}
	w = env.do(t, http.MethodGet, "/api/v1/users/raven", "", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("gated profile: %d", w.Code)
	}
	w = env.do(t, http.MethodGet, "/api/v1/users/raven", "", env.other, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("logged-in profile: %d", w.Code)
	}
}

func TestDeleteAccountSelf(t *testing.T) {
	env, blobs, _ := newAccountEnv(t)
	ctx := context.Background()

	// other has an avatar, a published skill, a session
	otherID := userID(t, env, "other")
	if err := blobs.PutAvatar(otherID, bytes.NewReader(widePNG(t, 64, 64))); err != nil {
		t.Fatal(err)
	}
	if _, err := env.stores.DB.ExecContext(ctx,
		`UPDATE user SET has_avatar = 1 WHERE id = ?`, otherID); err != nil {
		t.Fatal(err)
	}
	env.publishAs(t, env.other, "my-thing", "d", "1.0.0", nil, "")

	w := env.do(t, http.MethodDelete, "/api/v1/account", "", env.other, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete account: %d %s", w.Code, w.Body)
	}
	// session invalid
	w = env.do(t, http.MethodGet, "/api/v1/auth/me", "", env.other, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("session after delete: %d", w.Code)
	}
	// user gone, avatar group gone, artifact taken down
	var n int
	_ = env.stores.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user WHERE id = ?`, otherID).Scan(&n)
	if n != 0 {
		t.Fatal("user should be deleted")
	}
	if _, err := os.Stat(blobs.AvatarPath(otherID, "")); !os.IsNotExist(err) {
		t.Fatalf("avatar group should be deleted: %v", err)
	}
	var status string
	_ = env.stores.DB.QueryRowContext(ctx,
		`SELECT status FROM skill WHERE name = 'my-thing'`).Scan(&status)
	if status != store.SkillStatusTakenDown {
		t.Fatalf("artifact should be taken_down, got %q", status)
	}
}

func TestDeleteAccountFounderForbidden(t *testing.T) {
	env, _, _ := newAccountEnv(t)
	if _, err := env.stores.DB.ExecContext(context.Background(),
		`UPDATE user SET is_founder = 1 WHERE username = 'raven'`); err != nil {
		t.Fatal(err)
	}
	w := env.do(t, http.MethodDelete, "/api/v1/account", "", env.cookie, nil)
	if w.Code != http.StatusForbidden ||
		decode(t, w)["error"].(map[string]any)["code"] != "founder_protected" {
		t.Fatalf("founder delete: %d %s", w.Code, w.Body)
	}
}

func TestOverviewIncludesAvatars(t *testing.T) {
	env, blobs, _ := newAccountEnv(t)
	otherID := userID(t, env, "other")

	w := env.do(t, http.MethodGet, "/api/v1/admin/overview", "", env.cookie, nil)
	before := decode(t, w)["stats"].(map[string]any)["storage_bytes"].(float64)

	if err := blobs.PutAvatar(otherID, bytes.NewReader(widePNG(t, 128, 64))); err != nil {
		t.Fatal(err)
	}
	w = env.do(t, http.MethodGet, "/api/v1/admin/overview", "", env.cookie, nil)
	after := decode(t, w)["stats"].(map[string]any)["storage_bytes"].(float64)
	if after <= before {
		t.Fatalf("storage_bytes should grow with avatars: %v -> %v", before, after)
	}
}

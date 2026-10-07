package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/config"
	"github.com/ravenmk2/rune-market/internal/hub"
	"github.com/ravenmk2/rune-market/internal/store"
)

// skillsEnv is a fully wired normal-mode engine with real SQLite and blob storage.
type skillsEnv struct {
	engine *gin.Engine
	stores *store.Stores
	cookie *http.Cookie // logged-in raven (admin)
	other  *http.Cookie // logged-in regular user
}

func newSkillsEnv(t *testing.T) *skillsEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
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
	authSvc := auth.NewService(stores.Users, stores.Sessions, stores.Settings)
	blobs := blob.New(dataDir)
	deps := testDeps(t)
	skillsH := NewSkillsHandler(hub.NewSkills(db, store.DialectSQLite, blobs), stores.Settings, blobs, deps.Logger)

	r := NewNormalEngine(deps, authSvc, skillsH, nil, nil, nil)

	env := &skillsEnv{engine: r, stores: stores}
	env.cookie = env.registerLogin(t, "raven", "Raven")
	env.other = env.registerLogin(t, "other", "Other")
	// elevate raven to admin
	if _, err := db.ExecContext(ctx,
		`UPDATE user SET role = 'admin' WHERE username = 'raven'`); err != nil {
		t.Fatal(err)
	}
	return env
}

func (e *skillsEnv) registerLogin(t *testing.T, username, nickname string) *http.Cookie {
	t.Helper()
	w := e.do(t, http.MethodPost, "/api/v1/auth/register",
		`{"username":"`+username+`","nickname":"`+nickname+`","password":"secret123"}`, nil, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("register %s: %d %s", username, w.Code, w.Body)
	}
	w = e.do(t, http.MethodPost, "/api/v1/auth/login",
		`{"username":"`+username+`","password":"secret123"}`, nil, nil)
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func (e *skillsEnv) do(t *testing.T, method, path string, body string, cookie *http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if headers == nil {
		headers = map[string]string{}
	}
	if method != http.MethodGet {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func (e *skillsEnv) doRaw(t *testing.T, method, path string, body []byte, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Content-Type", "application/octet-stream")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

// zipContentsEqual compares two archives by entry names and contents, not
// raw bytes (zip headers carry timestamps with 2-second granularity).
func zipContentsEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	read := func(data []byte) map[string]string {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("open zip: %v", err)
		}
		out := map[string]string{}
		for _, f := range zr.File {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			content, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Fatal(err)
			}
			out[f.Name] = string(content)
		}
		return out
	}
	ma, mb := read(a), read(b)
	if len(ma) != len(mb) {
		return false
	}
	for name, content := range ma {
		if mb[name] != content {
			return false
		}
	}
	return true
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return m
}

// buildSkillZip returns a valid skill archive.
func buildSkillZip(t *testing.T, name, description string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	files := map[string]string{
		name + "/SKILL.md": "---\nname: " + name + "\ndescription: " + description +
			"\nlicense: MIT\nallowed-tools: Bash(python3:*) Read\n" +
			"metadata:\n  author: Raven\nwhen_to_use: testing\n---\n\n# Body\n",
		name + "/references/usage.md": "# Usage\nSome docs.\n",
	}
	for n, b := range files {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, b); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (e *skillsEnv) publish(t *testing.T, name, description, version, tags string) map[string]any {
	t.Helper()
	w := e.doRaw(t, http.MethodPost,
		"/api/v1/skills?version="+version+"&tags="+tags,
		buildSkillZip(t, name, description), e.cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("publish %s@%s: %d %s", name, version, w.Code, w.Body)
	}
	return decode(t, w)["skill"].(map[string]any)
}

func TestValidateEndpoint(t *testing.T) {
	env := newSkillsEnv(t)

	w := env.doRaw(t, http.MethodPost, "/api/v1/skills/validate",
		buildSkillZip(t, "pdf-processing", "Extract PDFs"), env.cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("validate: %d %s", w.Code, w.Body)
	}
	m := decode(t, w)
	checks, ok := m["checks"].([]any)
	if !ok || len(checks) == 0 {
		t.Fatalf("checks: %v", m)
	}
	for _, c := range checks {
		lvl := c.(map[string]any)["level"]
		if lvl == "error" {
			t.Fatalf("unexpected error check: %v", c)
		}
	}
	meta := m["metadata"].(map[string]any)
	if meta["name"] != "pdf-processing" || meta["file_count"].(float64) != 2 {
		t.Fatalf("metadata: %v", meta)
	}
	perms := meta["permissions"].([]any)
	if perms[0].(map[string]any)["tool"] != "Bash(python3:*)" ||
		perms[0].(map[string]any)["risk"] != "warn" {
		t.Fatalf("permissions: %v", perms)
	}
	harnesses := meta["harnesses"].([]any)
	if len(harnesses) != 1 || harnesses[0] != "claude-code" {
		t.Fatalf("harnesses: %v", harnesses)
	}
	if meta["sha256"].(string) == "" || meta["size"].(float64) <= 0 {
		t.Fatalf("hash/size: %v", meta)
	}

	// hostile package produces error-level checks but still HTTP 200
	w = env.doRaw(t, http.MethodPost, "/api/v1/skills/validate",
		[]byte("definitely not an archive............"), env.cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("validate hostile: %d", w.Code)
	}
	m = decode(t, w)
	var hasErr bool
	for _, c := range m["checks"].([]any) {
		if c.(map[string]any)["level"] == "error" {
			hasErr = true
		}
	}
	if !hasErr {
		t.Fatalf("expected error checks: %v", m)
	}

	// validate requires auth
	w = env.doRaw(t, http.MethodPost, "/api/v1/skills/validate",
		buildSkillZip(t, "x", "y"), nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous validate: %d", w.Code)
	}
}

func TestSkillLifecycleAPI(t *testing.T) {
	env := newSkillsEnv(t)

	skill := env.publish(t, "pdf-processing", "Extract PDFs", "1.0.0", "文档,处理")
	if skill["namespace"] != "raven" || skill["name"] != "pdf-processing" ||
		skill["status"] != "published" || skill["latest_version"] != "1.0.0" {
		t.Fatalf("skill: %v", skill)
	}
	owner := skill["owner"].(map[string]any)
	if owner["username"] != "raven" || owner["nickname"] != "Raven" {
		t.Fatalf("owner: %v", owner)
	}
	latest := skill["latest"].(map[string]any)
	if latest["version"] != "1.0.0" || latest["filename"] != "pdf-processing-1.0.0.zip" ||
		latest["author"] != "Raven" {
		t.Fatalf("latest: %v", latest)
	}

	// version conflict → 409
	w := env.doRaw(t, http.MethodPost, "/api/v1/skills?version=1.0.0",
		buildSkillZip(t, "pdf-processing", "Extract PDFs"), env.cookie)
	if w.Code != http.StatusConflict {
		t.Fatalf("version conflict: %d %s", w.Code, w.Body)
	}

	// invalid version → 400
	w = env.doRaw(t, http.MethodPost, "/api/v1/skills?version=v1",
		buildSkillZip(t, "pdf-processing", "Extract PDFs"), env.cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad version: %d", w.Code)
	}

	// new version
	env.publish(t, "pdf-processing", "Extract PDFs v2", "1.1.0", "")

	// list
	w = env.do(t, http.MethodGet, "/api/v1/skills", "", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	m := decode(t, w)
	if m["total"].(float64) != 1 || m["page_size"].(float64) != 20 {
		t.Fatalf("list: %v", m)
	}
	item := m["items"].([]any)[0].(map[string]any)
	if item["latest_version"] != "1.1.0" || item["summary"] != "Extract PDFs v2" {
		t.Fatalf("item: %v", item)
	}
	tags := item["tags"].([]any)
	if len(tags) != 0 {
		t.Fatalf("tags should be replaced by publish: %v", tags)
	}

	// q filter
	w = env.do(t, http.MethodGet, "/api/v1/skills?q=nomatch", "", nil, nil)
	if decode(t, w)["total"].(float64) != 0 {
		t.Fatalf("q filter failed")
	}

	// detail
	w = env.do(t, http.MethodGet, "/api/v1/skills/raven/pdf-processing", "", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("detail: %d", w.Code)
	}
	d := decode(t, w)["skill"].(map[string]any)
	if d["latest"].(map[string]any)["version"] != "1.1.0" {
		t.Fatalf("detail latest: %v", d)
	}

	// versions list
	w = env.do(t, http.MethodGet, "/api/v1/skills/raven/pdf-processing/versions", "", nil, nil)
	versions := decode(t, w)["items"].([]any)
	if len(versions) != 2 || versions[0].(map[string]any)["version"] != "1.1.0" {
		t.Fatalf("versions: %v", versions)
	}

	// single version
	w = env.do(t, http.MethodGet, "/api/v1/skills/raven/pdf-processing/versions/1.0.0", "", nil, nil)
	if decode(t, w)["version"].(map[string]any)["version"] != "1.0.0" {
		t.Fatalf("version 1.0.0: %v", w.Body)
	}

	// file tree
	w = env.do(t, http.MethodGet,
		"/api/v1/skills/raven/pdf-processing/versions/1.0.0/files", "", nil, nil)
	files := decode(t, w)["files"].([]any)
	if len(files) != 2 || files[0].(map[string]any)["path"] == "" {
		t.Fatalf("files: %v", files)
	}

	// single file content
	w = env.do(t, http.MethodGet,
		"/api/v1/skills/raven/pdf-processing/versions/1.0.0/file?path=pdf-processing/references/usage.md",
		"", nil, nil)
	fm := decode(t, w)
	if fm["content_type"] != "text" || !strings.Contains(fm["content"].(string), "Usage") {
		t.Fatalf("file: %v", fm)
	}
	// traversal rejected
	w = env.do(t, http.MethodGet,
		"/api/v1/skills/raven/pdf-processing/versions/1.0.0/file?path=../x", "", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("traversal: %d", w.Code)
	}

	// download increments the counter
	w = env.do(t, http.MethodGet,
		"/api/v1/skills/raven/pdf-processing/versions/1.0.0/download", "", nil, nil)
	if w.Code != http.StatusOK ||
		!strings.Contains(w.Header().Get("Content-Disposition"), "pdf-processing-1.0.0.zip") {
		t.Fatalf("download: %d %v", w.Code, w.Header())
	}
	if !zipContentsEqual(t, w.Body.Bytes(), buildSkillZip(t, "pdf-processing", "Extract PDFs")) {
		t.Fatal("download content mismatch")
	}
	w = env.do(t, http.MethodGet, "/api/v1/skills/raven/pdf-processing", "", nil, nil)
	if decode(t, w)["skill"].(map[string]any)["download_count"].(float64) != 1 {
		t.Fatal("download_count not incremented")
	}

	// edit tags (owner)
	w = env.do(t, http.MethodPut, "/api/v1/skills/"+skillID(t, env, "pdf-processing"),
		`{"tags":["文档","效率"]}`, env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("update tags: %d %s", w.Code, w.Body)
	}
	tags = decode(t, w)["skill"].(map[string]any)["tags"].([]any)
	if len(tags) != 2 {
		t.Fatalf("tags after edit: %v", tags)
	}
	// non-owner cannot edit
	w = env.do(t, http.MethodPut, "/api/v1/skills/"+skillID(t, env, "pdf-processing"),
		`{"tags":["x"]}`, env.other, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-owner edit: %d", w.Code)
	}
	// tag filter works
	w = env.do(t, http.MethodGet, "/api/v1/skills?tag=效率", "", nil, nil)
	if decode(t, w)["total"].(float64) != 1 {
		t.Fatalf("tag filter failed")
	}

	// takedown hides from public list and anonymous detail
	w = env.do(t, http.MethodPost, "/api/v1/skills/"+skillID(t, env, "pdf-processing")+"/takedown",
		"", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("takedown: %d %s", w.Code, w.Body)
	}
	w = env.do(t, http.MethodGet, "/api/v1/skills", "", nil, nil)
	if decode(t, w)["total"].(float64) != 0 {
		t.Fatal("taken-down skill still listed")
	}
	w = env.do(t, http.MethodGet, "/api/v1/skills/raven/pdf-processing", "", nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("anonymous detail of taken-down: %d", w.Code)
	}
	// owner still sees it
	w = env.do(t, http.MethodGet, "/api/v1/skills/raven/pdf-processing", "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("owner detail of taken-down: %d", w.Code)
	}
	// mine lists it with status
	w = env.do(t, http.MethodGet, "/api/v1/mine/skills", "", env.cookie, nil)
	mine := decode(t, w)["items"].([]any)
	if len(mine) != 1 || mine[0].(map[string]any)["status"] != "taken_down" {
		t.Fatalf("mine: %v", mine)
	}

	// restore
	w = env.do(t, http.MethodPost, "/api/v1/skills/"+skillID(t, env, "pdf-processing")+"/restore",
		"", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("restore: %d", w.Code)
	}
	w = env.do(t, http.MethodGet, "/api/v1/skills", "", nil, nil)
	if decode(t, w)["total"].(float64) != 1 {
		t.Fatal("restored skill not listed")
	}

	// delete
	w = env.do(t, http.MethodDelete, "/api/v1/skills/"+skillID(t, env, "pdf-processing"),
		"", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d", w.Code)
	}
	w = env.do(t, http.MethodGet, "/api/v1/skills/raven/pdf-processing", "", nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("detail after delete: %d", w.Code)
	}
}

func skillID(t *testing.T, env *skillsEnv, name string) string {
	t.Helper()
	var id string
	if err := env.stores.DB.QueryRowContext(context.Background(),
		`SELECT id FROM skill WHERE name = ?`, name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestArtifactReviewPending(t *testing.T) {
	env := newSkillsEnv(t)
	ctx := context.Background()
	if err := env.stores.Settings.Set(ctx, "artifact_review", "required"); err != nil {
		t.Fatal(err)
	}

	skill := env.publish(t, "needs-review", "d", "1.0.0", "")
	if skill["status"] != "pending" {
		t.Fatalf("status: %v", skill["status"])
	}
	// invisible publicly
	w := env.do(t, http.MethodGet, "/api/v1/skills", "", nil, nil)
	if decode(t, w)["total"].(float64) != 0 {
		t.Fatal("pending skill listed publicly")
	}
	// visible to owner
	w = env.do(t, http.MethodGet, "/api/v1/skills/raven/needs-review", "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("owner view pending: %d", w.Code)
	}
}

func TestAnonymousGating(t *testing.T) {
	env := newSkillsEnv(t)
	ctx := context.Background()
	env.publish(t, "gated", "d", "1.0.0", "")

	if err := env.stores.Settings.Set(ctx, "anonymous_browse", "false"); err != nil {
		t.Fatal(err)
	}
	w := env.do(t, http.MethodGet, "/api/v1/skills", "", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous browse off: %d", w.Code)
	}
	w = env.do(t, http.MethodGet, "/api/v1/skills", "", env.other, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("logged-in browse: %d", w.Code)
	}

	if err := env.stores.Settings.Set(ctx, "anonymous_browse", "true"); err != nil {
		t.Fatal(err)
	}
	if err := env.stores.Settings.Set(ctx, "anonymous_download", "false"); err != nil {
		t.Fatal(err)
	}
	w = env.do(t, http.MethodGet,
		"/api/v1/skills/raven/gated/versions/1.0.0/download", "", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous download off: %d", w.Code)
	}
	w = env.do(t, http.MethodGet,
		"/api/v1/skills/raven/gated/versions/1.0.0/download", "", env.other, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("logged-in download: %d", w.Code)
	}
}

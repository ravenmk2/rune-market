package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/config"
	"github.com/ravenmk2/rune-market/internal/secret"
	"github.com/ravenmk2/rune-market/internal/store"
)

func testStatic() fstest.MapFS {
	return fstest.MapFS{
		"index.html":      &fstest.MapFile{Data: []byte("<html>spa</html>")},
		"assets/app-1.js": &fstest.MapFile{Data: []byte("console.log(1)")},
	}
}

func testDeps(t *testing.T) Deps {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := logrus.New()
	logger.SetOutput(ioDiscard{})
	return Deps{
		DataDir: t.TempDir(),
		Version: "test",
		Logger:  logger,
		Static:  testStatic(),
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func do(t *testing.T, h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func csrf() map[string]string {
	return map[string]string{"X-Requested-With": "XMLHttpRequest"}
}

func TestSecurityHeaders(t *testing.T) {
	deps := testDeps(t)
	r := NewNormalEngine(deps, nil, nil)
	w := do(t, r, http.MethodGet, "/", "", nil)
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options: %q", got)
	}
	if got := w.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options: %q", got)
	}
	if w.Header().Get("Referrer-Policy") == "" {
		t.Fatal("Referrer-Policy missing")
	}
}

func TestSPAFallback(t *testing.T) {
	deps := testDeps(t)
	r := NewNormalEngine(deps, nil, nil)

	// static hit
	w := do(t, r, http.MethodGet, "/assets/app-1.js", "", nil)
	if w.Code != http.StatusOK || w.Body.String() != "console.log(1)" {
		t.Fatalf("static: %d %q", w.Code, w.Body)
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("assets should be immutable: %q", w.Header().Get("Cache-Control"))
	}

	// SPA fallback
	w = do(t, r, http.MethodGet, "/s/runemarket/pdf-processing", "", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "spa") {
		t.Fatalf("fallback: %d %q", w.Code, w.Body)
	}

	// unknown API is a JSON 404, not the SPA
	w = do(t, r, http.MethodGet, "/api/v1/nope", "", nil)
	if w.Code != http.StatusNotFound ||
		!strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("api 404: %d %q", w.Code, w.Body)
	}

	// setup routes are absent in normal mode
	w = do(t, r, http.MethodGet, "/api/v1/setup/status", "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("setup status in normal mode: %d", w.Code)
	}
}

func TestSetupModeRedirects(t *testing.T) {
	deps := testDeps(t)
	setupSvc := NewSetupService(deps.DataDir, deps.Logger,
		func(context.Context, *sql.DB, *config.Config) error { return nil })
	r := NewSetupEngine(deps, setupSvc)

	// wizard page itself serves the SPA
	w := do(t, r, http.MethodGet, "/setup", "", nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "spa") {
		t.Fatalf("/setup: %d %q", w.Code, w.Body)
	}
	// everything else redirects
	for _, p := range []string{"/", "/skills", "/api/v1/auth/me"} {
		w = do(t, r, http.MethodGet, p, "", nil)
		if w.Code != http.StatusFound || w.Header().Get("Location") != "/setup" {
			t.Fatalf("%s: %d loc=%q", p, w.Code, w.Header().Get("Location"))
		}
	}
	// static assets still served (wizard needs its JS)
	w = do(t, r, http.MethodGet, "/assets/app-1.js", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("assets in setup mode: %d", w.Code)
	}
}

// TestSetupStateMachine walks the whole wizard: status → database → admin →
// hot switch to normal mode → register/login works, setup routes 404.
func TestSetupStateMachine(t *testing.T) {
	deps := testDeps(t)
	completed := false
	var sw *Switcher
	onComplete := func(ctx context.Context, db *sql.DB, cfg *config.Config) error {
		stores := store.NewStores(db, cfg.Database.Driver)
		authSvc := auth.NewService(stores.Users, stores.Sessions, stores.Settings)
		sw.Swap(NewNormalEngine(deps, authSvc, nil))
		completed = true
		return nil
	}
	setupSvc := NewSetupService(deps.DataDir, deps.Logger, onComplete)
	t.Cleanup(func() {
		if setupSvc.db != nil {
			_ = setupSvc.db.Close()
		}
	})
	sw = NewSwitcher(NewSetupEngine(deps, setupSvc))

	statusStep := func() string {
		w := do(t, sw, http.MethodGet, "/api/v1/setup/status", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("status: %d %s", w.Code, w.Body)
		}
		var body struct {
			Mode string `json:"mode"`
			Step string `json:"step"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Mode != "setup" {
			t.Fatalf("mode: %q", body.Mode)
		}
		return body.Step
	}
	if step := statusStep(); step != SetupStepDatabase {
		t.Fatalf("initial step: %q", step)
	}

	// admin before database is rejected
	w := do(t, sw, http.MethodPost, "/api/v1/setup/admin",
		`{"username":"founder","nickname":"Boss","password":"secret123"}`, csrf())
	if w.Code != http.StatusConflict {
		t.Fatalf("admin first: %d %s", w.Code, w.Body)
	}

	// sqlite test connection is trivially ok
	w = do(t, sw, http.MethodPost, "/api/v1/setup/database/test",
		`{"driver":"sqlite"}`, csrf())
	if w.Code != http.StatusOK {
		t.Fatalf("db test: %d %s", w.Code, w.Body)
	}

	// invalid driver rejected
	w = do(t, sw, http.MethodPost, "/api/v1/setup/database",
		`{"driver":"postgres"}`, csrf())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad driver: %d %s", w.Code, w.Body)
	}

	// mysql with missing fields rejected
	w = do(t, sw, http.MethodPost, "/api/v1/setup/database",
		`{"driver":"mysql","host":"","database":"runemarket","username":"u","password":"p"}`, csrf())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("mysql missing host: %d %s", w.Code, w.Body)
	}

	// mysql test connection failure surfaces as a request error
	w = do(t, sw, http.MethodPost, "/api/v1/setup/database/test",
		`{"driver":"mysql","host":"127.0.0.1:1","database":"runemarket","username":"u","password":"p"}`, csrf())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("mysql unreachable: %d %s", w.Code, w.Body)
	}

	// save sqlite database config (empty data_dir resolves to the server data dir)
	w = do(t, sw, http.MethodPost, "/api/v1/setup/database",
		`{"driver":"sqlite","data_dir":""}`, csrf())
	if w.Code != http.StatusOK {
		t.Fatalf("save database: %d %s", w.Code, w.Body)
	}
	if step := statusStep(); step != SetupStepAdmin {
		t.Fatalf("step after database: %q", step)
	}
	// config.toml was written at this step (§8.1), with resolved data_dir
	cfg, err := config.Load(deps.DataDir)
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if cfg.Database.Driver != "sqlite" || cfg.Database.DataDir != deps.DataDir {
		t.Fatalf("saved config: %+v", cfg.Database)
	}

	// weak password rejected
	w = do(t, sw, http.MethodPost, "/api/v1/setup/admin",
		`{"username":"founder","nickname":"Boss","password":"short"}`, csrf())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("weak password: %d %s", w.Code, w.Body)
	}

	// create founder → completes installation and swaps engine
	w = do(t, sw, http.MethodPost, "/api/v1/setup/admin",
		`{"username":"founder","nickname":"Boss","password":"secret123"}`, csrf())
	if w.Code != http.StatusOK {
		t.Fatalf("create admin: %d %s", w.Code, w.Body)
	}
	if !completed {
		t.Fatal("onComplete was not called")
	}

	// setup routes now 404 on the swapped normal engine
	w = do(t, sw, http.MethodGet, "/api/v1/setup/status", "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("setup status after completion: %d", w.Code)
	}
	// no more redirect: normal SPA fallback
	w = do(t, sw, http.MethodGet, "/", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("root after completion: %d", w.Code)
	}

	// founder can log in and is an admin founder
	w = do(t, sw, http.MethodPost, "/api/v1/auth/login",
		`{"username":"founder","password":"secret123"}`, csrf())
	if w.Code != http.StatusOK {
		t.Fatalf("founder login: %d %s", w.Code, w.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	user := body["user"].(map[string]any)
	if user["role"] != "admin" || user["is_founder"] != true {
		t.Fatalf("founder flags: %v", user)
	}

	// secret was generated (32-byte key)
	secretBytes, err := secret.LoadOrCreate(deps.DataDir)
	if err != nil || len(secretBytes) != 32 {
		t.Fatalf("secret: len=%d err=%v", len(secretBytes), err)
	}
}

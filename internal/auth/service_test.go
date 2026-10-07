package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ravenmk2/rune-market/internal/config"
	"github.com/ravenmk2/rune-market/internal/store"
)

// testEnv wires a real SQLite-backed store set into a Gin engine.
type testEnv struct {
	engine  *gin.Engine
	api     *gin.RouterGroup
	stores  *store.Stores
	service *Service
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx := context.Background()

	cfg := &config.Config{Database: config.Database{
		Driver:  store.DialectSQLite,
		DataDir: t.TempDir(),
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
	svc := NewService(stores.Users, stores.Sessions, stores.Settings)

	r := gin.New()
	api := r.Group("/api/v1", CSRFProtect(), svc.Resolve())
	svc.RegisterRoutes(api.Group("/auth"))
	return &testEnv{engine: r, api: api, stores: stores, service: svc}
}

func (e *testEnv) do(t *testing.T, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

// postJSON sends a CSRF-compliant POST.
func (e *testEnv) postJSON(t *testing.T, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	headers := map[string]string{"X-Requested-With": "XMLHttpRequest"}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func sessionCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookieName {
			return c
		}
	}
	t.Fatal("no session cookie set")
	return nil
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return m
}

func TestRegisterLoginMeLogout(t *testing.T) {
	env := newTestEnv(t)

	w := env.postJSON(t, "/api/v1/auth/register",
		`{"username":"alice","nickname":"Alice","password":"secret123"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", w.Code, w.Body)
	}
	body := decodeBody(t, w)
	user := body["user"].(map[string]any)
	if user["username"] != "alice" || user["status"] != "active" || user["role"] != "user" {
		t.Fatalf("unexpected user: %v", user)
	}
	// frontend contract: these fields must be present on every user payload
	for _, field := range []string{
		"id", "username", "nickname", "role", "status", "has_avatar", "is_founder", "updated_at",
	} {
		if _, ok := user[field]; !ok {
			t.Fatalf("user payload missing field %q: %v", field, user)
		}
	}
	if _, hasHash := user["password_hash"]; hasHash {
		t.Fatal("password_hash leaked in response")
	}
	// register does not auto-login
	if sessionCookiePresent(w) {
		t.Fatal("register should not set a session cookie")
	}

	// duplicate username
	w = env.postJSON(t, "/api/v1/auth/register",
		`{"username":"alice","nickname":"Alice2","password":"secret123"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate register: %d %s", w.Code, w.Body)
	}

	// bad password
	w = env.postJSON(t, "/api/v1/auth/login",
		`{"username":"alice","password":"wrong-password"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: %d %s", w.Code, w.Body)
	}

	// login
	w = env.postJSON(t, "/api/v1/auth/login",
		`{"username":"alice","password":"secret123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("login: %d %s", w.Code, w.Body)
	}
	cookie := sessionCookie(t, w)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie flags: %+v", cookie)
	}

	// me
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	env.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("me: %d %s", w.Code, w.Body)
	}
	if got := decodeBody(t, w)["user"].(map[string]any)["username"]; got != "alice" {
		t.Fatalf("me username: %v", got)
	}

	// logout invalidates the session server-side
	w = env.postJSON(t, "/api/v1/auth/logout", ``, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("logout: %d %s", w.Code, w.Body)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	env.engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: %d", w.Code)
	}
}

func sessionCookiePresent(w *httptest.ResponseRecorder) bool {
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookieName && c.Value != "" {
			return true
		}
	}
	return false
}

func TestRegisterValidation(t *testing.T) {
	env := newTestEnv(t)
	cases := []struct {
		name string
		body string
	}{
		{"bad username uppercase", `{"username":"Alice","nickname":"A","password":"secret123"}`},
		{"bad username double hyphen", `{"username":"a--b","nickname":"A","password":"secret123"}`},
		{"bad username trailing hyphen", `{"username":"ab-","nickname":"A","password":"secret123"}`},
		{"short password", `{"username":"alice","nickname":"A","password":"short"}`},
		{"empty nickname", `{"username":"alice","nickname":"  ","password":"secret123"}`},
		{"not json", `{`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := env.postJSON(t, "/api/v1/auth/register", tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d %s", w.Code, w.Body)
			}
		})
	}
}

func TestRegistrationModes(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	if err := env.stores.Settings.Set(ctx, "registration_mode", "approval"); err != nil {
		t.Fatal(err)
	}
	w := env.postJSON(t, "/api/v1/auth/register",
		`{"username":"pending-user","nickname":"P","password":"secret123"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("approval register: %d %s", w.Code, w.Body)
	}
	if got := decodeBody(t, w)["user"].(map[string]any)["status"]; got != "pending" {
		t.Fatalf("expected pending, got %v", got)
	}
	w = env.postJSON(t, "/api/v1/auth/login",
		`{"username":"pending-user","password":"secret123"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("pending login: %d %s", w.Code, w.Body)
	}

	if err := env.stores.Settings.Set(ctx, "registration_mode", "closed"); err != nil {
		t.Fatal(err)
	}
	w = env.postJSON(t, "/api/v1/auth/register",
		`{"username":"third","nickname":"T","password":"secret123"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("closed register: %d %s", w.Code, w.Body)
	}
}

func TestCSRFProtection(t *testing.T) {
	env := newTestEnv(t)
	// no X-Requested-With header
	w := env.do(t, http.MethodPost, "/api/v1/auth/login",
		`{"username":"x","password":"y12345678"}`, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF header, got %d", w.Code)
	}
	if got := decodeBody(t, w)["error"].(map[string]any)["code"]; got != "csrf_failed" {
		t.Fatalf("code: %v", got)
	}
	// GET passes without the header
	w = env.do(t, http.MethodGet, "/api/v1/auth/me", "", nil)
	if w.Code == http.StatusForbidden {
		t.Fatalf("GET should not require CSRF header: %d", w.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	env := newTestEnv(t)
	var last *httptest.ResponseRecorder
	for i := 0; i < 11; i++ {
		last = env.postJSON(t, "/api/v1/auth/login",
			`{"username":"nobody","password":"secret123"}`)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("11th attempt: expected 429, got %d", last.Code)
	}
}

func TestSessionSlidingRenewal(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	w := env.postJSON(t, "/api/v1/auth/register",
		`{"username":"alice","nickname":"A","password":"secret123"}`)
	if w.Code != http.StatusCreated {
		t.Fatal(w.Body)
	}
	w = env.postJSON(t, "/api/v1/auth/login",
		`{"username":"alice","password":"secret123"}`)
	cookie := sessionCookie(t, w)

	// shrink the stored expiry below the renewal threshold
	sess, err := env.stores.Sessions.GetByTokenHash(ctx, hashToken(cookie.Value))
	if err != nil {
		t.Fatal(err)
	}
	soon := store.Now().Add(6 * 24 * time.Hour)
	if _, err := env.stores.DB.ExecContext(ctx,
		`UPDATE session SET expires_at = ? WHERE id = ?`, soon, sess.ID); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	env.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("me: %d", w.Code)
	}
	sess, _ = env.stores.Sessions.GetByTokenHash(ctx, hashToken(cookie.Value))
	if time.Until(sess.ExpiresAt) < 13*24*time.Hour {
		t.Fatalf("session not renewed, expires at %v", sess.ExpiresAt)
	}
}

func TestFounderProtection(t *testing.T) {
	founder := &store.User{IsFounder: true}
	if err := EnsureMutable(founder); err != ErrFounderProtected {
		t.Fatalf("expected ErrFounderProtected, got %v", err)
	}
	if err := EnsureMutable(&store.User{}); err != nil {
		t.Fatalf("regular user should be mutable: %v", err)
	}
}

func TestRequireAdmin(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	// elevate alice to admin, bob stays a regular user
	mkUser := func(username, role string) *http.Cookie {
		t.Helper()
		now := store.Now()
		hash, _ := HashPassword("secret123")
		u := &store.User{
			ID: store.NewID(), Username: username, Nickname: username,
			PasswordHash: hash, Role: role, Status: store.StatusActive,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := env.stores.Users.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		w := env.postJSON(t, "/api/v1/auth/login",
			`{"username":"`+username+`","password":"secret123"}`)
		return sessionCookie(t, w)
	}
	adminCookie := mkUser("root-admin", store.RoleAdmin)
	userCookie := mkUser("plain-user", store.RoleUser)

	env.api.GET("/admin-probe", env.service.RequireAdmin(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for _, tc := range []struct {
		name   string
		cookie *http.Cookie
		want   int
	}{
		{"anonymous", nil, http.StatusUnauthorized},
		{"regular user", userCookie, http.StatusForbidden},
		{"admin", adminCookie, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin-probe", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			w := httptest.NewRecorder()
			env.engine.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, w.Code)
			}
		})
	}
}

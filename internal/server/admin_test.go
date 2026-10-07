package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/hub"
	"github.com/ravenmk2/rune-market/internal/store"
)

// newAdminEnv builds an engine with all handlers including admin.
// raven is an admin (and founder when the test marks it), other is a user.
func newAdminEnv(t *testing.T) *skillsEnv {
	env := newSkillsEnv(t)
	stores := env.stores
	authSvc := auth.NewService(stores.Users, stores.Sessions, stores.Settings)
	blobs := blob.New(t.TempDir())
	deps := testDeps(t)
	skillsSvc := hub.NewSkills(stores.DB, store.DialectSQLite, blobs)
	designsSvc := hub.NewDesigns(stores.DB, store.DialectSQLite, blobs)
	skillsH := NewSkillsHandler(skillsSvc, stores.Settings, blobs, deps.Logger)
	designsH := NewDesignsHandler(designsSvc, stores.Settings, blobs, deps.Logger)
	adminH := NewAdminHandler(stores, skillsSvc, designsSvc, deps.Logger,
		"test-version", deps.DataDir, store.DialectSQLite)
	env.engine = NewNormalEngine(deps, authSvc, skillsH, designsH, adminH,
		NewSiteHandler(stores.Settings))
	return env
}

func userID(t *testing.T, env *skillsEnv, username string) string {
	t.Helper()
	var id string
	if err := env.stores.DB.QueryRowContext(context.Background(),
		`SELECT id FROM user WHERE username = ?`, username).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func login(t *testing.T, env *skillsEnv, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	return env.do(t, http.MethodPost, "/api/v1/auth/login",
		`{"username":"`+username+`","password":"`+password+`"}`, nil, nil)
}

func TestAdminPermissionMatrix(t *testing.T) {
	env := newAdminEnv(t)

	// anonymous → 401
	w := env.do(t, http.MethodGet, "/api/v1/admin/overview", "", nil, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", w.Code)
	}
	// regular user → 403
	w = env.do(t, http.MethodGet, "/api/v1/admin/overview", "", env.other, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("regular user: %d", w.Code)
	}
	// admin → 200 with the contract shape
	w = env.do(t, http.MethodGet, "/api/v1/admin/overview", "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin: %d %s", w.Code, w.Body)
	}
	m := decode(t, w)
	stats := m["stats"].(map[string]any)
	todos := m["todos"].(map[string]any)
	system := m["system"].(map[string]any)
	if stats["users"].(float64) != 2 || stats["storage_bytes"].(float64) != 0 {
		t.Fatalf("stats: %v", stats)
	}
	if todos["pending_users"].(float64) != 0 {
		t.Fatalf("todos: %v", todos)
	}
	if system["database"] != "sqlite" || system["version"] != "test-version" ||
		system["registration_mode"] != "open" || system["secret_created"] != false {
		t.Fatalf("system: %v", system)
	}
}

func TestApprovalFlow(t *testing.T) {
	env := newAdminEnv(t)
	ctx := context.Background()
	if err := env.stores.Settings.Set(ctx, "registration_mode", "approval"); err != nil {
		t.Fatal(err)
	}

	w := env.do(t, http.MethodPost, "/api/v1/auth/register",
		`{"username":"newbie","nickname":"N","password":"secret123"}`, nil, nil)
	if w.Code != http.StatusCreated ||
		decode(t, w)["user"].(map[string]any)["status"] != "pending" {
		t.Fatalf("register: %d %s", w.Code, w.Body)
	}
	if w := login(t, env, "newbie", "secret123"); w.Code != http.StatusForbidden {
		t.Fatalf("pending login: %d", w.Code)
	}

	// admin user list filtered by status
	w = env.do(t, http.MethodGet, "/api/v1/admin/users?status=pending", "", env.cookie, nil)
	m := decode(t, w)
	if m["total"].(float64) != 1 {
		t.Fatalf("pending filter: %v", m)
	}
	item := m["items"].([]any)[0].(map[string]any)
	for _, f := range []string{"id", "username", "nickname", "role", "is_founder", "status", "has_avatar", "created_at"} {
		if _, ok := item[f]; !ok {
			t.Fatalf("user item missing %q: %v", f, item)
		}
	}
	id := item["id"].(string)

	// approve → can log in
	w = env.do(t, http.MethodPost, "/api/v1/admin/users/"+id+"/approve", "", env.cookie, nil)
	if w.Code != http.StatusOK || decode(t, w)["user"].(map[string]any)["status"] != "active" {
		t.Fatalf("approve: %d %s", w.Code, w.Body)
	}
	if w := login(t, env, "newbie", "secret123"); w.Code != http.StatusOK {
		t.Fatalf("login after approve: %d %s", w.Code, w.Body)
	}
	// approving again is a 400
	w = env.do(t, http.MethodPost, "/api/v1/admin/users/"+id+"/approve", "", env.cookie, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("re-approve: %d", w.Code)
	}
}

func TestAdminCreateUser(t *testing.T) {
	env := newAdminEnv(t)

	w := env.do(t, http.MethodPost, "/api/v1/admin/users",
		`{"username":"staff","nickname":"Staff","password":"secret123","role":"admin"}`, env.cookie, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	if decode(t, w)["user"].(map[string]any)["role"] != "admin" {
		t.Fatalf("role: %v", w.Body)
	}
	if w := login(t, env, "staff", "secret123"); w.Code != http.StatusOK {
		t.Fatalf("created user login: %d", w.Code)
	}

	// invalid role / duplicate / bad username
	for _, tc := range []struct{ body string }{
		{`{"username":"x1","nickname":"X","password":"secret123","role":"superadmin"}`},
		{`{"username":"staff","nickname":"X","password":"secret123","role":"user"}`},
		{`{"username":"Bad Name","nickname":"X","password":"secret123","role":"user"}`},
	} {
		w = env.do(t, http.MethodPost, "/api/v1/admin/users", tc.body, env.cookie, nil)
		if w.Code == http.StatusCreated {
			t.Fatalf("should reject %s", tc.body)
		}
	}
}

func TestDisableEnableResetPassword(t *testing.T) {
	env := newAdminEnv(t)
	id := userID(t, env, "other")

	// other logs in, gets a session
	w := login(t, env, "other", "secret123")
	var otherCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			otherCookie = c
		}
	}

	// disable → 403 on login, old session invalidated
	w = env.do(t, http.MethodPost, "/api/v1/admin/users/"+id+"/disable", "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", w.Code, w.Body)
	}
	if w := login(t, env, "other", "secret123"); w.Code != http.StatusForbidden {
		t.Fatalf("disabled login: %d", w.Code)
	}
	w = env.do(t, http.MethodGet, "/api/v1/auth/me", "", otherCookie, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("session should be invalidated on disable: %d", w.Code)
	}

	// enable → reset-password → temp password works, old one does not
	w = env.do(t, http.MethodPost, "/api/v1/admin/users/"+id+"/enable", "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("enable: %d", w.Code)
	}
	w = env.do(t, http.MethodPost, "/api/v1/admin/users/"+id+"/reset-password", "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", w.Code, w.Body)
	}
	temp := decode(t, w)["temporary_password"].(string)
	if len(temp) != 12 {
		t.Fatalf("temp password length: %q", temp)
	}
	if w := login(t, env, "other", "secret123"); w.Code != http.StatusUnauthorized {
		t.Fatalf("old password should fail: %d", w.Code)
	}
	if w := login(t, env, "other", temp); w.Code != http.StatusOK {
		t.Fatalf("temp password login: %d %s", w.Code, w.Body)
	}
}

func TestFounderAndSelfProtection(t *testing.T) {
	env := newAdminEnv(t)
	ctx := context.Background()
	// mark raven as founder
	if _, err := env.stores.DB.ExecContext(ctx,
		`UPDATE user SET is_founder = 1 WHERE username = 'raven'`); err != nil {
		t.Fatal(err)
	}
	ravenID := userID(t, env, "raven")
	otherID := userID(t, env, "other")

	// promote other to admin so they can attempt founder mutations
	w := env.do(t, http.MethodPost, "/api/v1/admin/users/"+otherID+"/role",
		`{"role":"admin"}`, env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("promote: %d %s", w.Code, w.Body)
	}
	// re-login other to pick up the new role in session resolution
	w = login(t, env, "other", "secret123")
	var otherAdmin *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			otherAdmin = c
		}
	}

	// founder cannot be disabled / demoted / deleted
	for _, tc := range []struct {
		method, path, body string
	}{
		{http.MethodPost, "/api/v1/admin/users/" + ravenID + "/disable", ""},
		{http.MethodPost, "/api/v1/admin/users/" + ravenID + "/role", `{"role":"user"}`},
		{http.MethodDelete, "/api/v1/admin/users/" + ravenID, ""},
	} {
		w = env.do(t, tc.method, tc.path, tc.body, otherAdmin, nil)
		if w.Code != http.StatusForbidden ||
			decode(t, w)["error"].(map[string]any)["code"] != "founder_protected" {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body)
		}
	}

	// self-protection: other admin disables / demotes self
	w = env.do(t, http.MethodPost, "/api/v1/admin/users/"+otherID+"/disable", "", otherAdmin, nil)
	if w.Code != http.StatusForbidden ||
		decode(t, w)["error"].(map[string]any)["code"] != "self_protected" {
		t.Fatalf("self disable: %d %s", w.Code, w.Body)
	}
	w = env.do(t, http.MethodPost, "/api/v1/admin/users/"+otherID+"/role",
		`{"role":"user"}`, otherAdmin, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("self demote: %d %s", w.Code, w.Body)
	}
	// but promoting self to admin is allowed (no-op upgrade)
	w = env.do(t, http.MethodPost, "/api/v1/admin/users/"+otherID+"/role",
		`{"role":"admin"}`, otherAdmin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("self promote: %d %s", w.Code, w.Body)
	}
}

func TestDeleteUserCascade(t *testing.T) {
	env := newAdminEnv(t)
	ctx := context.Background()
	// other publishes a skill
	w := env.doRaw(t, http.MethodPost, "/api/v1/skills?version=1.0.0",
		buildSkillZip(t, "owned-thing", "d"), env.other)
	if w.Code != http.StatusCreated {
		t.Fatalf("publish: %d %s", w.Code, w.Body)
	}
	otherID := userID(t, env, "other")

	w = env.do(t, http.MethodDelete, "/api/v1/admin/users/"+otherID, "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body)
	}
	// user gone, sessions gone, artifact taken_down (not deleted)
	var n int
	_ = env.stores.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user WHERE id = ?`, otherID).Scan(&n)
	if n != 0 {
		t.Fatal("user should be deleted")
	}
	_ = env.stores.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM session WHERE user_id = ?`, otherID).Scan(&n)
	if n != 0 {
		t.Fatal("sessions should cascade")
	}
	var status string
	_ = env.stores.DB.QueryRowContext(ctx,
		`SELECT status FROM skill WHERE name = 'owned-thing'`).Scan(&status)
	if status != store.SkillStatusTakenDown {
		t.Fatalf("artifact should be taken_down, got %q", status)
	}
}

func TestAdminArtifactsOfficialApprove(t *testing.T) {
	env := newAdminEnv(t)
	ctx := context.Background()
	if err := env.stores.Settings.Set(ctx, "artifact_review", "required"); err != nil {
		t.Fatal(err)
	}

	skill := env.publish(t, "reviewed-skill", "d", "1.0.0", "")
	if skill["status"] != "pending" {
		t.Fatalf("expected pending: %v", skill["status"])
	}
	sid := skill["id"].(string)

	// admin list shows pending items
	w := env.do(t, http.MethodGet, "/api/v1/admin/skills?status=pending", "", env.cookie, nil)
	if decode(t, w)["total"].(float64) != 1 {
		t.Fatalf("admin skills pending filter: %v", w.Body)
	}

	// approve → published + visible in marketplace
	w = env.do(t, http.MethodPost, "/api/v1/admin/skills/"+sid+"/approve", "", env.cookie, nil)
	if w.Code != http.StatusOK ||
		decode(t, w)["skill"].(map[string]any)["status"] != "published" {
		t.Fatalf("approve skill: %d %s", w.Code, w.Body)
	}
	w = env.do(t, http.MethodGet, "/api/v1/skills", "", nil, nil)
	if decode(t, w)["total"].(float64) != 1 {
		t.Fatal("approved skill should be listed")
	}
	// re-approve → 400
	w = env.do(t, http.MethodPost, "/api/v1/admin/skills/"+sid+"/approve", "", env.cookie, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("re-approve: %d", w.Code)
	}

	// official flag on/off
	w = env.do(t, http.MethodPost, "/api/v1/admin/skills/"+sid+"/official", "", env.cookie, nil)
	if w.Code != http.StatusOK ||
		decode(t, w)["skill"].(map[string]any)["official"] != true {
		t.Fatalf("official: %d %s", w.Code, w.Body)
	}
	w = env.do(t, http.MethodGet, "/api/v1/skills?official=true", "", nil, nil)
	if decode(t, w)["total"].(float64) != 1 {
		t.Fatal("official filter should match")
	}
	w = env.do(t, http.MethodPost, "/api/v1/admin/skills/"+sid+"/unofficial", "", env.cookie, nil)
	if decode(t, w)["skill"].(map[string]any)["official"] != false {
		t.Fatalf("unofficial: %s", w.Body)
	}

	// non-admin cannot mark
	w = env.do(t, http.MethodPost, "/api/v1/admin/skills/"+sid+"/official", "", env.other, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin official: %d", w.Code)
	}
}

func TestAdminSettings(t *testing.T) {
	env := newAdminEnv(t)

	// GET returns the 8 contract keys
	w := env.do(t, http.MethodGet, "/api/v1/admin/settings", "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get settings: %d", w.Code)
	}
	settings := decode(t, w)["settings"].(map[string]any)
	for _, k := range []string{"site_name", "site_description", "page_size",
		"registration_mode", "artifact_review", "upload_max_mb",
		"anonymous_browse", "anonymous_download"} {
		if _, ok := settings[k]; !ok {
			t.Fatalf("missing setting %q: %v", k, settings)
		}
	}

	// partial update
	w = env.do(t, http.MethodPut, "/api/v1/admin/settings",
		`{"settings":{"site_name":"RuneMarket 内网版","registration_mode":"closed","page_size":"50"}}`,
		env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("put settings: %d %s", w.Code, w.Body)
	}
	settings = decode(t, w)["settings"].(map[string]any)
	if settings["site_name"] != "RuneMarket 内网版" ||
		settings["registration_mode"] != "closed" || settings["page_size"] != "50" {
		t.Fatalf("after put: %v", settings)
	}
	// untouched key preserved
	if settings["artifact_review"] != "none" {
		t.Fatalf("artifact_review should be preserved: %v", settings)
	}

	// invalid values rejected
	for _, body := range []string{
		`{"settings":{"registration_mode":"sometimes"}}`,
		`{"settings":{"page_size":"0"}}`,
		`{"settings":{"page_size":"500"}}`,
		`{"settings":{"upload_max_mb":"abc"}}`,
		`{"settings":{"anonymous_browse":"yes"}}`,
		`{"settings":{"unknown_key":"x"}}`,
	} {
		w = env.do(t, http.MethodPut, "/api/v1/admin/settings", body, env.cookie, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("should reject %s: %d", body, w.Code)
		}
	}
	// rejected update did not persist
	w = env.do(t, http.MethodGet, "/api/v1/admin/settings", "", env.cookie, nil)
	if decode(t, w)["settings"].(map[string]any)["registration_mode"] != "closed" {
		t.Fatal("failed validation should not persist")
	}

	// JSON scalars are normalized (SPA sends native booleans/numbers)
	w = env.do(t, http.MethodPut, "/api/v1/admin/settings",
		`{"settings":{"page_size":30,"anonymous_browse":false,"site_name":"标量版"}}`,
		env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("scalar settings: %d %s", w.Code, w.Body)
	}
	settings = decode(t, w)["settings"].(map[string]any)
	if settings["page_size"] != "30" || settings["anonymous_browse"] != "false" ||
		settings["site_name"] != "标量版" {
		t.Fatalf("after scalar put: %v", settings)
	}

	// public site identity endpoint reflects site_name anonymously
	w = env.do(t, http.MethodGet, "/api/v1/site", "", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("site endpoint: %d", w.Code)
	}
	if got := decode(t, w)["site_name"]; got != "标量版" {
		t.Fatalf("site_name: %v", got)
	}
	// non-scalar values are rejected
	w = env.do(t, http.MethodPut, "/api/v1/admin/settings",
		`{"settings":{"site_name":["x"]}}`, env.cookie, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("non-scalar should be rejected: %d", w.Code)
	}
}

func TestSecretRegenerate(t *testing.T) {
	env := newAdminEnv(t)

	// both users hold sessions; regenerate invalidates all of them
	w := env.do(t, http.MethodPost, "/api/v1/admin/settings/secret/regenerate", "", env.cookie, nil)
	if w.Code != http.StatusOK || decode(t, w)["ok"] != true {
		t.Fatalf("regenerate: %d %s", w.Code, w.Body)
	}
	w = env.do(t, http.MethodGet, "/api/v1/auth/me", "", env.cookie, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("admin session should be invalidated: %d", w.Code)
	}
	w = env.do(t, http.MethodGet, "/api/v1/auth/me", "", env.other, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("user session should be invalidated: %d", w.Code)
	}
	// login still works afterwards
	if w := login(t, env, "raven", "secret123"); w.Code != http.StatusOK {
		t.Fatalf("login after regenerate: %d", w.Code)
	}
}

func TestAdminUsersSearch(t *testing.T) {
	env := newAdminEnv(t)
	w := env.do(t, http.MethodGet, "/api/v1/admin/users?q=raven", "", env.cookie, nil)
	m := decode(t, w)
	if m["total"].(float64) != 1 {
		t.Fatalf("q filter: %v", m)
	}
	item := m["items"].([]any)[0].(map[string]any)
	if item["username"] != "raven" || item["role"] != "admin" {
		t.Fatalf("item: %v", item)
	}
	if _, leaked := item["password_hash"]; leaked {
		t.Fatal("password_hash leaked")
	}
}

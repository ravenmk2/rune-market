package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/blob"
	"github.com/ravenmk2/rune-market/internal/hub"
	"github.com/ravenmk2/rune-market/internal/store"
)

// wireDesignsPlaceholder removed; see newSkillsEnvWithDesigns below.

const sampleDesign = `# Acme Design

## Overview

Brand system for Acme apps.

## Colors

Primary #1a73e8, surface #ffffff.

## Typography

Inter.
`

func makeTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func makeTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// uploadImage does POST /images and returns the decoded response.
func (e *skillsEnv) uploadImage(t *testing.T, img []byte) map[string]any {
	t.Helper()
	w := e.doRaw(t, http.MethodPost, "/api/v1/images", img, e.cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("image upload: %d %s", w.Code, w.Body)
	}
	return decode(t, w)
}

// publishDesign does the JSON POST /designs.
func (e *skillsEnv) publishDesign(t *testing.T, name, summary, version string, tags []string, desktop, mobile string) map[string]any {
	t.Helper()
	body := map[string]any{
		"content": sampleDesign, "name": name, "summary": summary, "version": version,
	}
	if len(tags) > 0 {
		body["tags"] = tags
	}
	if desktop != "" {
		body["preview_desktop"] = desktop
	}
	if mobile != "" {
		body["preview_mobile"] = mobile
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	w := e.do(t, http.MethodPost, "/api/v1/designs", string(raw), e.cookie, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("publish design %s@%s: %d %s", name, version, w.Code, w.Body)
	}
	return decode(t, w)["design"].(map[string]any)
}

func TestDesignLifecycleAPI(t *testing.T) {
	env := newSkillsEnvWithDesigns(t)

	// upload preview images via POST /images
	desktop := env.uploadImage(t, makeTestPNG(t, 1280, 800))
	mobile := env.uploadImage(t, makeTestPNG(t, 375, 812))
	desktopSHA := desktop["sha256"].(string)
	mobileSHA := mobile["sha256"].(string)
	if desktop["ext"] != "png" || desktop["url"] != "/images/"+desktopSHA+".png" ||
		desktop["thumb_url"] != "/images/"+desktopSHA+"_640.png" {
		t.Fatalf("image response: %v", desktop)
	}

	// non-image rejected
	w := env.doRaw(t, http.MethodPost, "/api/v1/images", []byte("not an image at all"), env.cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("non-image: %d", w.Code)
	}
	// anonymous upload rejected
	w = env.doRaw(t, http.MethodPost, "/api/v1/images", makeTestPNG(t, 8, 8), nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous image upload: %d", w.Code)
	}

	// validate: weak validation, checks only (JSON dry run)
	validateBody := func(name, content string) string {
		raw, err := json.Marshal(map[string]string{"name": name, "content": content})
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	w = env.do(t, http.MethodPost, "/api/v1/designs/validate",
		validateBody("acme-ui", sampleDesign), env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("validate: %d %s", w.Code, w.Body)
	}
	m := decode(t, w)
	if _, hasMeta := m["metadata"]; hasMeta {
		t.Fatal("design validate must not include metadata")
	}
	checks := m["checks"].([]any)
	var hasWarn, hasColorInfo bool
	for _, c := range checks {
		cm := c.(map[string]any)
		if cm["level"] == "warn" && strings.Contains(cm["detail"].(string), "Spacing") {
			hasWarn = true
		}
		if cm["title"] == "颜色定义" && strings.Contains(cm["detail"].(string), "2 个") {
			hasColorInfo = true
		}
		if cm["level"] == "error" {
			t.Fatalf("unexpected error check: %v", cm)
		}
	}
	if !hasWarn || !hasColorInfo {
		t.Fatalf("checks: %v", checks)
	}

	// empty content → error level (JSON transport normalizes invalid UTF-8,
	// so the empty-file check is the reachable error path)
	w = env.do(t, http.MethodPost, "/api/v1/designs/validate",
		`{"name":"x","content":""}`, env.cookie, nil)
	m = decode(t, w)
	if m["checks"].([]any)[0].(map[string]any)["level"] != "error" {
		t.Fatalf("empty content: %v", m)
	}
	// malformed JSON → 400
	w = env.do(t, http.MethodPost, "/api/v1/designs/validate", `{bad`, env.cookie, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad json: %d", w.Code)
	}

	// publish with previews (JSON body)
	d := env.publishDesign(t, "acme-ui", "Acme design system", "1.0.0",
		[]string{"品牌", "深色"}, desktopSHA, mobileSHA)
	if d["namespace"] != "raven" || d["name"] != "acme-ui" || d["status"] != "published" {
		t.Fatalf("design: %v", d)
	}
	if d["preview_thumb_url"] != "/images/"+desktopSHA+"_640.png" {
		t.Fatalf("thumb url: %v", d["preview_thumb_url"])
	}
	latest := d["latest"].(map[string]any)
	if latest["version"] != "1.0.0" ||
		latest["preview_desktop_url"] != "/images/"+desktopSHA+".png" ||
		latest["preview_mobile_url"] != "/images/"+mobileSHA+".png" {
		t.Fatalf("latest: %v", latest)
	}
	sid := d["id"].(string)

	// version conflict
	w = env.do(t, http.MethodPost, "/api/v1/designs",
		`{"content":"# Acme Design\n\n## Overview\n\nx\n","name":"acme-ui","version":"1.0.0"}`, env.cookie, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("conflict: %d %s", w.Code, w.Body)
	}

	// unknown preview blob → 400
	w = env.do(t, http.MethodPost, "/api/v1/designs",
		`{"content":"# Acme Design\n\n## Overview\n\nx\n","name":"acme-ui","version":"9.9.9",`+
			`"preview_desktop":"0000000000000000000000000000000000000000000000000000000000000000"}`, env.cookie, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown preview: %d %s", w.Code, w.Body)
	}

	// list item contract
	w = env.do(t, http.MethodGet, "/api/v1/designs", "", nil, nil)
	items := decode(t, w)["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("list: %v", w.Body)
	}
	item := items[0].(map[string]any)
	for _, field := range []string{"id", "namespace", "name", "summary", "official",
		"status", "tags", "latest_version", "download_count", "updated_at",
		"preview_thumb_url", "owner"} {
		if _, ok := item[field]; !ok {
			t.Fatalf("list item missing %q: %v", field, item)
		}
	}

	// versions + single version
	w = env.do(t, http.MethodGet, "/api/v1/designs/raven/acme-ui/versions", "", nil, nil)
	versions := decode(t, w)["items"].([]any)
	if len(versions) != 1 {
		t.Fatalf("versions: %v", w.Body)
	}
	if versions[0].(map[string]any)["preview_desktop_url"] != "/images/"+desktopSHA+".png" {
		t.Fatalf("version preview url: %v", versions[0])
	}
	w = env.do(t, http.MethodGet, "/api/v1/designs/raven/acme-ui/versions/1.0.0", "", nil, nil)
	if decode(t, w)["version"].(map[string]any)["sha256"] == "" {
		t.Fatalf("version: %v", w.Body)
	}

	// content
	w = env.do(t, http.MethodGet, "/api/v1/designs/raven/acme-ui/versions/1.0.0/content", "", nil, nil)
	cm := decode(t, w)
	if cm["content"] != sampleDesign || cm["version"] != "1.0.0" || cm["sha256"] == "" {
		t.Fatalf("content: %v", cm)
	}

	// download: text/markdown, attachment, counter
	w = env.do(t, http.MethodGet, "/api/v1/designs/raven/acme-ui/versions/1.0.0/download", "", nil, nil)
	if w.Code != http.StatusOK ||
		!strings.Contains(w.Header().Get("Content-Type"), "text/markdown") ||
		!strings.Contains(w.Header().Get("Content-Disposition"), "acme-ui-1.0.0.md") ||
		w.Body.String() != sampleDesign {
		t.Fatalf("download: %d %v", w.Code, w.Header())
	}
	w = env.do(t, http.MethodGet, "/api/v1/designs/raven/acme-ui", "", nil, nil)
	if decode(t, w)["design"].(map[string]any)["download_count"].(float64) != 1 {
		t.Fatal("download_count not incremented")
	}

	// images: original, thumb, immutable cache, 404
	for _, p := range []string{"/images/" + desktopSHA + ".png", "/images/" + desktopSHA + "_640.png"} {
		w = env.do(t, http.MethodGet, p, "", nil, nil)
		if w.Code != http.StatusOK ||
			!strings.Contains(w.Header().Get("Cache-Control"), "immutable") ||
			w.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("image %s: %d %v", p, w.Code, w.Header())
		}
	}
	// a JPEG upload keeps its format and is served as image/jpeg
	jpegImg := env.uploadImage(t, makeTestJPEG(t, 800, 600))
	jpegSHA := jpegImg["sha256"].(string)
	if jpegImg["ext"] != "jpg" || jpegImg["url"] != "/images/"+jpegSHA+".jpg" {
		t.Fatalf("jpeg response: %v", jpegImg)
	}
	w = env.do(t, http.MethodGet, "/images/"+jpegSHA+".jpg", "", nil, nil)
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("jpeg serve: %d %v", w.Code, w.Header())
	}
	// names failing the sha regex are rejected (no path traversal surface)
	w = env.do(t, http.MethodGet, "/images/notasha.png", "", nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("bad name: %d", w.Code)
	}
	w = env.do(t, http.MethodGet, "/images/"+desktopSHA+"_999.png", "", nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unsupported derivative: %d", w.Code)
	}
	w = env.do(t, http.MethodGet,
		"/images/0000000000000000000000000000000000000000000000000000000000000000.png", "", nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing image: %d", w.Code)
	}

	// update summary + tags (owner), forbidden for non-owner
	w = env.do(t, http.MethodPut, "/api/v1/designs/"+sid,
		`{"summary":"Better summary","tags":["品牌"]}`, env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	d = decode(t, w)["design"].(map[string]any)
	if d["summary"] != "Better summary" || len(d["tags"].([]any)) != 1 {
		t.Fatalf("after update: %v", d)
	}
	w = env.do(t, http.MethodPut, "/api/v1/designs/"+sid, `{"tags":["x"]}`, env.other, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-owner update: %d", w.Code)
	}

	// takedown visibility, mine, restore, delete
	w = env.do(t, http.MethodPost, "/api/v1/designs/"+sid+"/takedown", "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("takedown: %d", w.Code)
	}
	w = env.do(t, http.MethodGet, "/api/v1/designs", "", nil, nil)
	if decode(t, w)["total"].(float64) != 0 {
		t.Fatal("taken-down design still listed")
	}
	w = env.do(t, http.MethodGet, "/api/v1/mine/designs", "", env.cookie, nil)
	mine := decode(t, w)["items"].([]any)
	if len(mine) != 1 || mine[0].(map[string]any)["status"] != "taken_down" {
		t.Fatalf("mine: %v", mine)
	}
	w = env.do(t, http.MethodPost, "/api/v1/designs/"+sid+"/restore", "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("restore: %d", w.Code)
	}
	w = env.do(t, http.MethodDelete, "/api/v1/designs/"+sid, "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body)
	}
	w = env.do(t, http.MethodGet, "/api/v1/designs/raven/acme-ui", "", nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("detail after delete: %d", w.Code)
	}

	// preview blob file still exists (upload holds one ref after delete releases its ref)
	var refs int
	if err := env.stores.DB.QueryRowContext(context.Background(),
		"SELECT ref_count FROM `blob` WHERE sha256 = ?", desktopSHA).Scan(&refs); err != nil {
		t.Fatal(err)
	}
	if refs != 1 {
		t.Fatalf("preview refs after delete: %d", refs)
	}
}

// newSkillsEnvWithDesigns rebuilds the engine with both handlers wired.
func newSkillsEnvWithDesigns(t *testing.T) *skillsEnv {
	env := newSkillsEnv(t)
	stores := env.stores
	authSvc := auth.NewService(stores.Users, stores.Sessions, stores.Settings)
	blobs := blob.New(t.TempDir())
	deps := testDeps(t)
	skillsH := NewSkillsHandler(hub.NewSkills(stores.DB, store.DialectSQLite, blobs), stores.Settings, blobs, deps.Logger)
	designsH := NewDesignsHandler(hub.NewDesigns(stores.DB, store.DialectSQLite, blobs), stores.Settings, blobs, deps.Logger)
	env.engine = NewNormalEngine(deps, authSvc, skillsH, designsH, nil, nil, nil)
	return env
}

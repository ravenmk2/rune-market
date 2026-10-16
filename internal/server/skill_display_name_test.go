package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestSkillDisplayNameLifecycle(t *testing.T) {
	env := newSkillsEnv(t)
	// Older clients omit display_name: new skills default to their package name.
	sk := env.publish(t, "skill-creator", "Create skills", "1.0.0", "")
	if sk["display_name"] != "skill-creator" {
		t.Fatalf("default display_name: %v", sk)
	}
	id := sk["id"].(string)
	updateURL := "/api/v1/skills/" + id
	w := env.do(t, http.MethodPut, updateURL, `{"display_name":"  技能创建者  "}`, env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	// Publishing a version without display_name preserves the edited name.
	sk = env.publish(t, "skill-creator", "Create more skills", "1.1.0", "")
	if sk["display_name"] != "技能创建者" || sk["name"] != "skill-creator" {
		t.Fatalf("republish: %v", sk)
	}
	for _, path := range []string{
		"/api/v1/skills?q=技能创建者",
		"/api/v1/skills?q=skill-creator",
		"/api/v1/mine/skills",
	} {
		w = env.do(t, http.MethodGet, path, "", env.cookie, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("list %s: %d %s", path, w.Code, w.Body)
		}
		items := decode(t, w)["items"].([]any)
		if len(items) != 1 || items[0].(map[string]any)["display_name"] != "技能创建者" {
			t.Fatalf("list %s: %v", path, items)
		}
	}
	// Invalid edits must leave the existing display name intact.
	for _, name := range []string{"", "   ", strings.Repeat("长", 65)} {
		body, _ := json.Marshal(map[string]string{"display_name": name})
		w = env.do(t, http.MethodPut, updateURL, string(body), env.cookie, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid name: %d %s", w.Code, w.Body)
		}
	}
	w = env.do(t, http.MethodPut, updateURL, `{"display_name":"别人的名称"}`, env.other, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-owner edit: %d %s", w.Code, w.Body)
	}
	w = env.do(t, http.MethodGet, "/api/v1/skills/raven/skill-creator", "", nil, nil)
	if w.Code != http.StatusOK || decode(t, w)["skill"].(map[string]any)["display_name"] != "技能创建者" {
		t.Fatalf("detail after rejected edits: %d %s", w.Code, w.Body)
	}
}

func TestPublishSkillDisplayName(t *testing.T) {
	env := newSkillsEnv(t)
	archive := env.uploadArchive(t, buildSkillZip(t, "skill-creator", "Create skills"), env.cookie)
	for i, name := range []string{strings.Repeat("长", 65), "  技能创建者  ", strings.Repeat("技", 64)} {
		body, _ := json.Marshal(map[string]string{
			"archive": archive, "version": "1.0." + strconv.Itoa(i), "display_name": name,
		})
		w := env.do(t, http.MethodPost, "/api/v1/skills", string(body), env.cookie, nil)
		if i == 0 {
			if w.Code != http.StatusBadRequest {
				t.Fatalf("long publish name: %d %s", w.Code, w.Body)
			}
			continue
		}
		if w.Code != http.StatusCreated {
			t.Fatalf("publish name: %d %s", w.Code, w.Body)
		}
		sk := decode(t, w)["skill"].(map[string]any)
		if sk["display_name"] != strings.TrimSpace(name) || sk["name"] != "skill-creator" {
			t.Fatalf("publish response: %v", sk)
		}
	}
}

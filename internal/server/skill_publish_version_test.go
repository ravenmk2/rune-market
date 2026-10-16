package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/ravenmk2/rune-market/internal/store"
)

func TestPublishBoundSkillVersionAPI(t *testing.T) {
	env := newSkillsEnv(t)
	sk := env.publish(t, "skill-creator", "first description", "1.2.3", "tools")
	id := sk["id"].(string)
	archive := env.uploadArchive(t, buildSkillZip(t, "skill-creator", "new description"), env.cookie)
	wrongArchive := env.uploadArchive(t, buildSkillZip(t, "different-skill", "wrong package"), env.cookie)
	for _, tc := range []struct {
		name, target, archive, version string
		cookie                         *http.Cookie
		status                         int
		code                           string
	}{
		{"wrong package", id, wrongArchive, "1.2.4", env.cookie, http.StatusBadRequest, "invalid_argument"},
		{"missing target", store.NewID(), archive, "1.2.4", env.cookie, http.StatusNotFound, "not_found"},
		{"not owner", id, archive, "1.2.4", env.other, http.StatusForbidden, "forbidden"},
		{"duplicate", id, archive, "1.2.3", env.cookie, http.StatusConflict, "version_exists"},
		{"lower version", id, archive, "1.2.2", env.cookie, http.StatusBadRequest, "invalid_argument"},
		{"new prerelease", id, archive, "1.2.4-beta.1", env.cookie, http.StatusCreated, ""},
		{"stable release", id, archive, "1.2.4", env.cookie, http.StatusCreated, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"skill_id": tc.target, "archive": tc.archive, "version": tc.version})
			w := env.do(t, http.MethodPost, "/api/v1/skills", string(body), tc.cookie, nil)
			if w.Code != tc.status {
				t.Fatalf("got %d %s, want %d", w.Code, w.Body, tc.status)
			}
			response := decode(t, w)
			if tc.code != "" {
				if response["error"].(map[string]any)["code"] != tc.code {
					t.Fatalf("error response: %v", response)
				}
				return
			}
			got := response["skill"].(map[string]any)
			if got["id"] != id || got["name"] != "skill-creator" || got["display_name"] != "skill-creator" {
				t.Fatalf("skill changed identity: %v", got)
			}
			if tags := got["tags"].([]any); len(tags) != 1 || tags[0] != "tools" {
				t.Fatalf("omitted tags were not preserved: %v", tags)
			}
		})
	}
}

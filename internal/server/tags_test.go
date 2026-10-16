package server

import (
	"context"
	"net/http"
	"reflect"
	"testing"
)

func TestPublishedTagVocabulary(t *testing.T) {
	env := newSkillsEnvWithDesigns(t)
	env.publishAs(t, env.cookie, "visible-one", "Visible", "1.0.0", []string{"shared", "alpha"}, "")
	hidden := env.publishAs(t, env.cookie, "hidden-one", "Hidden", "1.0.0", []string{"secret"}, "")
	env.publishAs(t, env.cookie, "visible-two", "Visible", "1.0.0", []string{"shared", "beta"}, "")
	env.publishDesign(t, "design-one", "Design", "1.0.0", []string{"light", "shared"}, "", "")
	w := env.do(t, http.MethodPost, "/api/v1/skills/"+hidden["id"].(string)+"/takedown", "", env.cookie, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("takedown: %d %s", w.Code, w.Body)
	}
	for _, tc := range []struct {
		path string
		want []any
	}{
		{"/api/v1/skills/tags", []any{"alpha", "beta", "shared"}},
		{"/api/v1/skills/tags?q=absent&tag=alpha&page=99", []any{"alpha", "beta", "shared"}},
		{"/api/v1/designs/tags", []any{"light", "shared"}},
	} {
		w := env.do(t, http.MethodGet, tc.path, "", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body)
		}
		if got := decode(t, w)["items"]; !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: got %v want %v", tc.path, got, tc.want)
		}
	}
	if err := env.stores.Settings.Set(context.Background(), "anonymous_browse", "false"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/skills/tags", "/api/v1/designs/tags"} {
		w := env.do(t, http.MethodGet, path, "", nil, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous %s: %d", path, w.Code)
		}
		w = env.do(t, http.MethodGet, path, "", env.cookie, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("authenticated %s: %d", path, w.Code)
		}
	}
}

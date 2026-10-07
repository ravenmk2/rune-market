package server

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ravenmk2/rune-market/internal/auth"
	"github.com/ravenmk2/rune-market/internal/store"
)

// apiSettings is the shared settings reader for marketplace handlers
// (skills and designs share the same gating knobs, §6.3).
type apiSettings struct {
	settings *store.SettingStore
}

func (s apiSettings) getStr(ctx context.Context, key, def string) string {
	v, err := s.settings.Get(ctx, key)
	if err != nil {
		return def
	}
	return v
}

func (s apiSettings) getBool(ctx context.Context, key string, def bool) bool {
	v, err := strconv.ParseBool(s.getStr(ctx, key, strconv.FormatBool(def)))
	if err != nil {
		return def
	}
	return v
}

func (s apiSettings) getInt(ctx context.Context, key string, def int) int {
	v, err := strconv.Atoi(s.getStr(ctx, key, strconv.Itoa(def)))
	if err != nil || v <= 0 {
		return def
	}
	return v
}

// requireBrowseAuth enforces anonymous_browse=off (§8.3).
func (s apiSettings) requireBrowseAuth(c *gin.Context) bool {
	if s.getBool(c.Request.Context(), "anonymous_browse", true) {
		return true
	}
	if auth.CurrentUser(c) != nil {
		return true
	}
	auth.Error(c, http.StatusUnauthorized, "unauthenticated", "login required")
	return false
}

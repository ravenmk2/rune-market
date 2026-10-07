package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ravenmk2/rune-market/internal/store"
)

// SiteHandler exposes the public site identity (§6.3 site_name /
// site_description) so the SPA can brand itself without admin rights.
type SiteHandler struct {
	apiSettings
}

func NewSiteHandler(settings *store.SettingStore) *SiteHandler {
	return &SiteHandler{apiSettings: apiSettings{settings: settings}}
}

func (h *SiteHandler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/site", h.site)
}

func (h *SiteHandler) site(c *gin.Context) {
	ctx := c.Request.Context()
	c.JSON(http.StatusOK, gin.H{
		"site_name":        h.getStr(ctx, "site_name", "RuneMarket"),
		"site_description": h.getStr(ctx, "site_description", ""),
	})
}

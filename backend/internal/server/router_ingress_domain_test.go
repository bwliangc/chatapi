//go:build unit

package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIngressDomainsFollowSavedSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newStubSettingRepo()
	repo.SetAll(map[string]string{
		service.SettingKeyAPIBaseURL:      "https://primary.example.com/v1",
		service.SettingKeyCustomEndpoints: `[{"name":"Hong Kong","endpoint":"https://hk.example.com/v1"}]`,
	})
	cfg := &config.Config{}
	settings := service.NewSettingService(repo, cfg)
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{}, MCPOAuth: &handler.MCPOAuthHandler{}}
	pass := func(c *gin.Context) { c.Next() }
	router := server.SetupRouter(gin.New(), handlers, pass, pass, pass, pass, pass, pass, nil, nil, nil, settings, nil, cfg, nil)
	router.GET("/v1/test-ingress", func(c *gin.Context) {
		c.String(http.StatusOK, "%s", ctxkey.IngressDomainFromContext(c.Request.Context()))
	})
	check := func(host, want string) {
		t.Helper()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "https://"+host+"/v1/test-ingress", nil))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, want, rec.Body.String(), "request to %s", host)
	}
	check("primary.example.com", "primary.example.com")
	check("hk.example.com", "hk.example.com")
	check("sg.example.com", "")

	require.NoError(t, settings.UpdateSettings(t.Context(), &service.SystemSettings{
		APIBaseURL:      "https://primary.example.com/v1",
		CustomEndpoints: `[{"name":"Singapore","endpoint":"https://sg.example.com/v1"}]`,
	}))
	check("primary.example.com", "primary.example.com")
	check("hk.example.com", "")
	check("sg.example.com", "sg.example.com")
}

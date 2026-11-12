package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIngressDomainFromTrustedProxy(t *testing.T) {
	for _, tt := range []struct {
		name    string
		peer    string
		proxies []string
		headers []string
		want    string
	}{
		{"first node", "192.0.2.10:4096", []string{"192.0.2.10"}, []string{" HK.API.EXAMPLE.COM. "}, "hk.api.example.com"},
		{"second node", "[2001:db8::4]:4096", []string{"2001:db8::/64"}, []string{"sg.api.example.com"}, "sg.api.example.com"},
		{"untrusted peer", "192.0.2.11:4096", []string{"192.0.2.10"}, []string{"hk.api.example.com"}, ""},
		{"no trusted proxies", "192.0.2.10:4096", nil, []string{"hk.api.example.com"}, ""},
		{"invalid proxy list", "192.0.2.10:4096", []string{"192.0.2.10", "invalid"}, []string{"hk.api.example.com"}, ""},
		{"missing header", "192.0.2.10:4096", []string{"192.0.2.10"}, nil, ""},
		{"repeated header", "192.0.2.10:4096", []string{"192.0.2.10"}, []string{"hk.api.example.com", "sg.api.example.com"}, ""},
		{"comma separated", "192.0.2.10:4096", []string{"192.0.2.10"}, []string{"hk.api.example.com,sg.api.example.com"}, ""},
		{"malformed value", "192.0.2.10:4096", []string{"192.0.2.10"}, []string{"https://hk.api.example.com/v1"}, ""},
		{"oversized label", "192.0.2.10:4096", []string{"192.0.2.10"}, []string{strings.Repeat("a", 64) + ".example.com"}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(IngressDomain(tt.proxies, nil))
			router.GET("/v1/models", func(c *gin.Context) {
				require.Empty(t, c.Request.Header.Values(ingressDomainHeader))
				c.String(http.StatusOK, "%s", ctxkey.IngressDomainFromContext(c.Request.Context()))
			})
			req := httptest.NewRequest(http.MethodGet, "http://backend.internal/v1/models", nil)
			req.RemoteAddr = tt.peer
			req.Header.Set("X-Forwarded-For", "192.0.2.10")
			req.Header.Set("X-Forwarded-Host", "spoofed.example.com")
			for _, value := range tt.headers {
				req.Header.Add(ingressDomainHeader, value)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tt.want, rec.Body.String())
		})
	}
}

func TestIngressDomainFromConfiguredAPIHost(t *testing.T) {
	domains := IngressDomainsFromURLs([]string{
		"https://HK.API.EXAMPLE.COM./v1", "https://sg.api.example.com:8443/v1",
		"https://192.0.2.10/v1", "*.api.example.com", "ftp://ftp.example.com",
	})
	for _, tt := range []struct {
		name, host, peer, marker, want string
	}{
		{"primary node without proxy metadata", "hk.api.example.com", "192.0.2.11:4096", "", "hk.api.example.com"},
		{"secondary node with a port", "SG.API.EXAMPLE.COM.:8443", "192.0.2.11:4096", "", "sg.api.example.com"},
		{"unknown subdomain", "other.api.example.com", "192.0.2.11:4096", "", ""},
		{"IP endpoint", "192.0.2.10", "192.0.2.11:4096", "", ""},
		{"non-HTTP endpoint", "ftp.example.com", "192.0.2.11:4096", "", ""},
		{"invalid port", "sg.api.example.com:wrong", "192.0.2.11:4096", "", ""},
		{"ignore forwarded host even from trusted proxy", "backend.internal", "192.0.2.10:4096", "", ""},
		{"ignore forged marker", "backend.internal", "192.0.2.11:4096", "hk.api.example.com", ""},
		{"prefer configured host over forged marker", "hk.api.example.com", "192.0.2.11:4096", "sg.api.example.com", "hk.api.example.com"},
		{"prefer trusted marker over configured host", "hk.api.example.com", "192.0.2.10:4096", "edge.api.example.com", "edge.api.example.com"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(IngressDomain([]string{"192.0.2.10"}, func() map[string]struct{} { return domains }))
			router.GET("/v1/models", func(c *gin.Context) {
				require.Empty(t, c.Request.Header.Values(ingressDomainHeader))
				c.String(http.StatusOK, "%s", ctxkey.IngressDomainFromContext(c.Request.Context()))
			})
			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			req.Host = tt.host
			req.RemoteAddr = tt.peer
			req.Header.Set("X-Forwarded-Host", "sg.api.example.com")
			if tt.marker != "" {
				req.Header.Set(ingressDomainHeader, tt.marker)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tt.want, rec.Body.String())
		})
	}
}

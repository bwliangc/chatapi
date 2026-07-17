//go:build embed

package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogoDataURL(t *testing.T) string {
	t.Helper()
	logo := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			logo.Set(x, y, color.RGBA{R: 240, G: 40, B: 80, A: 255})
		}
	}
	var output bytes.Buffer
	require.NoError(t, png.Encode(&output, logo))
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(output.Bytes())
}

func TestFrontendServerServesConfiguredPWAManifest(t *testing.T) {
	provider := &mockSettingsProvider{settings: map[string]string{
		"site_name": "Configured App",
		"site_logo": testLogoDataURL(t),
	}}
	server, err := NewFrontendServer(provider)
	require.NoError(t, err)
	router := gin.New()
	router.Use(server.Middleware())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/pwa/manifest.webmanifest", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	var manifest struct {
		Name      string `json:"name"`
		ShortName string `json:"short_name"`
		Icons     []struct {
			Src   string `json:"src"`
			Sizes string `json:"sizes"`
		} `json:"icons"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &manifest))
	assert.Equal(t, "Configured App", manifest.Name)
	assert.Equal(t, "Configured App", manifest.ShortName)
	assert.Equal(t, "/pwa/icon-192.png", manifest.Icons[0].Src)
	assert.Equal(t, "192x192", manifest.Icons[0].Sizes)

	legacy := httptest.NewRecorder()
	legacyRequest := httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil)
	router.ServeHTTP(legacy, legacyRequest)
	assert.Contains(t, legacy.Body.String(), `"name":"Configured App"`)
}

func TestFrontendServerServesConfiguredPWAIcons(t *testing.T) {
	provider := &mockSettingsProvider{settings: map[string]string{
		"site_name": "Configured App",
		"site_logo": testLogoDataURL(t),
	}}
	server, err := NewFrontendServer(provider)
	require.NoError(t, err)
	router := gin.New()
	router.Use(server.Middleware())

	for _, path := range []string{"/pwa/icon-180.png", "/pwa/icon-192.png", "/pwa/icon-512.png", "/pwa/icon-maskable-512.png"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
			decoded, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
			require.NoError(t, err)
			expectedSize := 512
			if path == "/pwa/icon-180.png" {
				expectedSize = 180
			} else if path == "/pwa/icon-192.png" {
				expectedSize = 192
			}
			assert.Equal(t, image.Rect(0, 0, expectedSize, expectedSize), decoded.Bounds())
		})
	}
}

func TestFrontendServerPWABrandingCacheInvalidation(t *testing.T) {
	provider := &mockSettingsProvider{settings: map[string]string{"site_name": "First"}}
	server, err := NewFrontendServer(provider)
	require.NoError(t, err)

	first := httptest.NewRecorder()
	firstContext, _ := gin.CreateTestContext(first)
	firstContext.Request = httptest.NewRequest(http.MethodGet, "/pwa/manifest.webmanifest", nil)
	server.servePWAManifest(firstContext)
	assert.Contains(t, first.Body.String(), `"name":"First"`)

	provider.settings = map[string]string{"site_name": "Second"}
	server.InvalidateCache()
	second := httptest.NewRecorder()
	secondContext, _ := gin.CreateTestContext(second)
	secondContext.Request = httptest.NewRequest(http.MethodGet, "/pwa/manifest.webmanifest", nil)
	server.servePWAManifest(secondContext)
	assert.Contains(t, second.Body.String(), `"name":"Second"`)
}

func TestFrontendServerBuildsFallbackIconFromCurrentLogo(t *testing.T) {
	provider := &mockSettingsProvider{settings: map[string]string{"site_name": "No Uploaded Logo"}}
	server, err := NewFrontendServer(provider)
	require.NoError(t, err)
	router := gin.New()
	router.Use(server.Middleware())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/pwa/icon-192.png", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	decoded, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 192, 192), decoded.Bounds())
}

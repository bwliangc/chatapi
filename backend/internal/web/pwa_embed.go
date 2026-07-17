//go:build embed

package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	xdraw "golang.org/x/image/draw"

	_ "image/gif"
	_ "image/jpeg"
)

const (
	defaultPWAName = "Sub2API"
	maxPWALogoSize = 1024 * 1024
)

type pwaBranding struct {
	Name string
	Logo string
}

type pwaBrandingCache struct {
	mu    sync.RWMutex
	value *pwaBranding
}

func (c *pwaBrandingCache) Get() (pwaBranding, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.value == nil {
		return pwaBranding{}, false
	}
	return *c.value, true
}

func (c *pwaBrandingCache) Set(value pwaBranding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = &value
}

func (c *pwaBrandingCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = nil
}

func (s *FrontendServer) getPWABranding(ctx context.Context) pwaBranding {
	if cached, ok := s.brandingCache.Get(); ok {
		return cached
	}

	branding := pwaBranding{Name: defaultPWAName}
	settings, err := s.settings.GetPublicSettingsForInjection(ctx)
	if err == nil {
		if payload, marshalErr := json.Marshal(settings); marshalErr == nil {
			var configured struct {
				SiteName string `json:"site_name"`
				SiteLogo string `json:"site_logo"`
			}
			if json.Unmarshal(payload, &configured) == nil {
				if name := strings.TrimSpace(configured.SiteName); name != "" {
					branding.Name = name
				}
				branding.Logo = strings.TrimSpace(configured.SiteLogo)
			}
		}
	}

	s.brandingCache.Set(branding)
	return branding
}

func (s *FrontendServer) tryServePWAResource(c *gin.Context, cleanPath string) bool {
	switch cleanPath {
	case "manifest.webmanifest", "pwa/manifest.webmanifest":
		s.servePWAManifest(c)
		return true
	case "pwa/icon-180.png":
		s.servePWAIcon(c, 180, false)
		return true
	case "pwa/icon-192.png":
		s.servePWAIcon(c, 192, false)
		return true
	case "pwa/icon-512.png":
		s.servePWAIcon(c, 512, false)
		return true
	case "pwa/icon-maskable-512.png":
		s.servePWAIcon(c, 512, true)
		return true
	default:
		return false
	}
}

func (s *FrontendServer) servePWAManifest(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	branding := s.getPWABranding(ctx)

	manifest := map[string]any{
		"id":                          "/",
		"name":                        branding.Name,
		"short_name":                  branding.Name,
		"description":                 "AI API Gateway management console",
		"lang":                        "zh-CN",
		"dir":                         "ltr",
		"start_url":                   "/home?source=pwa",
		"scope":                       "/",
		"display":                     "standalone",
		"display_override":            []string{"window-controls-overlay", "standalone"},
		"orientation":                 "any",
		"background_color":            "#080d1c",
		"theme_color":                 "#0e1426",
		"categories":                  []string{"business", "productivity", "utilities"},
		"prefer_related_applications": false,
		"icons": []map[string]string{
			{"src": "/pwa/icon-192.png", "sizes": "192x192", "type": "image/png", "purpose": "any"},
			{"src": "/pwa/icon-512.png", "sizes": "512x512", "type": "image/png", "purpose": "any"},
			{"src": "/pwa/icon-maskable-512.png", "sizes": "512x512", "type": "image/png", "purpose": "maskable"},
		},
		"shortcuts": []map[string]any{
			{"name": "Dashboard", "short_name": "Dashboard", "url": "/dashboard?source=pwa-shortcut", "icons": []map[string]string{{"src": "/pwa/icon-192.png", "sizes": "192x192"}}},
			{"name": "API Keys", "short_name": "API Keys", "url": "/keys?source=pwa-shortcut", "icons": []map[string]string{{"src": "/pwa/icon-192.png", "sizes": "192x192"}}},
		},
	}

	content, err := json.Marshal(manifest)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to build web app manifest")
		c.Abort()
		return
	}
	serveRevalidatingContent(c, "application/manifest+json; charset=utf-8", content)
}

func (s *FrontendServer) servePWAIcon(c *gin.Context, size int, maskable bool) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	branding := s.getPWABranding(ctx)

	source, sourceErr := decodePWADataImage(branding.Logo)
	if sourceErr != nil {
		source, sourceErr = s.loadDefaultPWALogo()
	}
	if sourceErr == nil {
		var output bytes.Buffer
		if png.Encode(&output, renderPWAIcon(source, size, maskable)) == nil {
			serveRevalidatingContent(c, "image/png", output.Bytes())
			return
		}
	}

	fallbackPath := "icons/pwa-512.png"
	switch {
	case maskable:
		fallbackPath = "icons/pwa-maskable-512.png"
	case size == 180:
		fallbackPath = "icons/apple-touch-icon.png"
	case size == 192:
		fallbackPath = "icons/pwa-192.png"
	}
	file, err := s.distFS.Open(fallbackPath)
	if err != nil {
		c.String(http.StatusNotFound, "PWA icon not found")
		c.Abort()
		return
	}
	defer func() { _ = file.Close() }()
	content, err := io.ReadAll(file)
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to read PWA icon")
		c.Abort()
		return
	}
	serveRevalidatingContent(c, "image/png", content)
}

func (s *FrontendServer) loadDefaultPWALogo() (image.Image, error) {
	file, err := s.distFS.Open("logo.png")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	decoded, _, err := image.Decode(io.LimitReader(file, maxPWALogoSize+1))
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func decodePWADataImage(value string) (image.Image, error) {
	header, payload, ok := strings.Cut(value, ",")
	if !ok || !strings.HasPrefix(strings.ToLower(header), "data:image/") || !strings.Contains(strings.ToLower(header), ";base64") {
		return nil, fmt.Errorf("site logo is not a base64 image data URL")
	}
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(payload))
	data, err := io.ReadAll(io.LimitReader(decoder, maxPWALogoSize+1))
	if err != nil || len(data) == 0 || len(data) > maxPWALogoSize {
		return nil, fmt.Errorf("site logo data is invalid or too large")
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func renderPWAIcon(source image.Image, size int, maskable bool) image.Image {
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	if maskable {
		draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 14, G: 20, B: 38, A: 255}}, image.Point{}, draw.Src)
	}

	sourceBounds := source.Bounds()
	maxContent := float64(size) * 0.9
	if maskable {
		maxContent = float64(size) * 0.72
	}
	scale := min(maxContent/float64(sourceBounds.Dx()), maxContent/float64(sourceBounds.Dy()))
	width := max(1, int(float64(sourceBounds.Dx())*scale))
	height := max(1, int(float64(sourceBounds.Dy())*scale))
	destination := image.Rect((size-width)/2, (size-height)/2, (size+width)/2, (size+height)/2)
	xdraw.CatmullRom.Scale(canvas, destination, source, sourceBounds, draw.Over, nil)
	return canvas
}

func serveRevalidatingContent(c *gin.Context, contentType string, content []byte) {
	hash := sha256.Sum256(content)
	etag := fmt.Sprintf(`"%x"`, hash[:8])
	c.Header("Cache-Control", "no-cache")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		c.Abort()
		return
	}
	c.Data(http.StatusOK, contentType, content)
	c.Abort()
}

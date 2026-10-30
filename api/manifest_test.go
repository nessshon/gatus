package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/TwiN/gatus/v5/config"
	"github.com/TwiN/gatus/v5/config/ui"
)

func TestManifestHandler_GetManifest(t *testing.T) {
	t.Parallel()
	custom := ui.GetDefaultConfig()
	custom.Header = `Status "TON" — Статус 🚀`
	custom.Description = `<b>Мониторинг</b> & "uptime"`
	custom.Favicon.Size192x192 = "/assets/icon-192.png"
	custom.Favicon.Size512x512 = "https://example.com/icon-512.png"
	scenarios := []struct {
		Name     string
		UI       *ui.Config
		Expected map[string]any
	}{
		{
			Name: "default",
			UI:   ui.GetDefaultConfig(),
			Expected: map[string]any{
				"id":               "gatus",
				"name":             "Gatus",
				"short_name":       "Gatus",
				"description":      "Gatus is an advanced automated status page that lets you monitor your applications and configure alerts to notify you if there's an issue",
				"lang":             "en",
				"scope":            "/",
				"start_url":        "/",
				"theme_color":      "#f7f9fb",
				"background_color": "#f7f9fb",
				"display":          "standalone",
				"icons": []any{
					map[string]any{"src": "/logo-192x192.png", "sizes": "192x192", "type": "image/png"},
					map[string]any{"src": "/logo-512x512.png", "sizes": "512x512", "type": "image/png"},
				},
			},
		},
		{
			Name: "custom",
			UI:   custom,
			Expected: map[string]any{
				"id":               "gatus",
				"name":             `Status "TON" — Статус 🚀`,
				"short_name":       `Status "TON" — Статус 🚀`,
				"description":      `<b>Мониторинг</b> & "uptime"`,
				"lang":             "en",
				"scope":            "/",
				"start_url":        "/",
				"theme_color":      "#f7f9fb",
				"background_color": "#f7f9fb",
				"display":          "standalone",
				"icons": []any{
					map[string]any{"src": "/assets/icon-192.png", "sizes": "192x192", "type": "image/png"},
					map[string]any{"src": "https://example.com/icon-512.png", "sizes": "512x512", "type": "image/png"},
				},
			},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			router := New(&config.Config{UI: scenario.UI}).Router()
			response, err := router.Test(httptest.NewRequest("GET", "/manifest.json", http.NoBody))
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", response.StatusCode)
			}
			if contentType := response.Header.Get("Content-Type"); contentType != "application/manifest+json" {
				t.Errorf("expected Content-Type application/manifest+json, got %s", contentType)
			}
			body, _ := io.ReadAll(response.Body)
			var actual map[string]any
			if err := json.Unmarshal(body, &actual); err != nil {
				t.Fatalf("expected valid JSON, got error %s for body %s", err.Error(), body)
			}
			if !reflect.DeepEqual(actual, scenario.Expected) {
				t.Errorf("expected %v, got %v", scenario.Expected, actual)
			}
		})
	}
}

package api

import (
	_ "embed"
	"html/template"

	"github.com/TwiN/gatus/v5/config"
	"github.com/TwiN/gatus/v5/config/ui"
	"github.com/TwiN/gatus/v5/storage/store"
	"github.com/TwiN/gatus/v5/storage/store/common/paging"
	static "github.com/TwiN/gatus/v5/web"
	"github.com/TwiN/logr"
	"github.com/gofiber/fiber/v2"
)

func SinglePageApplication(cfg *config.Config) fiber.Handler {
	uiConfig := cfg.UI
	hasOIDC := cfg.Security != nil && cfg.Security.OIDC != nil
	// Fallback when the store can't be read
	configuredCount := len(cfg.Endpoints) + len(cfg.ExternalEndpoints)
	return func(c *fiber.Ctx) error {
		vd := ui.ViewData{UI: uiConfig, OIDC: hasOIDC, PlaceholderCount: configuredCount}
		if placeholders, err := getPlaceholders(); err != nil {
			logr.Errorf("[api.SinglePageApplication] Failed to retrieve placeholders, falling back on the configured endpoint count: %s", err.Error())
		} else {
			vd.PlaceholderCount = len(placeholders)
			if cfg.Security == nil {
				vd.Placeholders = placeholders
			}
		}
		{
			themeFromCookie := string(c.Request().Header.Cookie("theme"))
			if len(themeFromCookie) > 0 {
				if themeFromCookie == "dark" {
					vd.Theme = "dark"
				}
			} else if uiConfig.IsDarkMode() { // Since there's no theme cookie, we'll rely on ui.DarkMode
				vd.Theme = "dark"
			}
		}
		t, err := template.ParseFS(static.FileSystem, static.IndexPath)
		if err != nil {
			// This should never happen, because ui.ValidateAndSetDefaults validates that the template works.
			logr.Errorf("[api.SinglePageApplication] Failed to parse template. This should never happen, because the template is validated on start. Error: %s", err.Error())
			return c.Status(500).SendString("Failed to parse template. This should never happen, because the template is validated on start.")
		}
		c.Set("Content-Type", "text/html")
		err = t.Execute(c, vd)
		if err != nil {
			// This should never happen, because ui.ValidateAndSetDefaults validates that the template works.
			logr.Errorf("[api.SinglePageApplication] Failed to execute template. This should never happen, because the template is validated on start. Error: %s", err.Error())
			return c.Status(500).SendString("Failed to parse template. This should never happen, because the template is validated on start.")
		}
		return c.SendStatus(200)
	}
}

// From the store: an external endpoint has no status until its first push
func getPlaceholders() ([]ui.Placeholder, error) {
	if value, exists := cache.Get("placeholders"); exists {
		return value.([]ui.Placeholder), nil
	}
	endpointStatuses, err := store.Get().GetAllEndpointStatuses(paging.NewEndpointStatusParams())
	if err != nil {
		return nil, err
	}
	placeholders := make([]ui.Placeholder, 0, len(endpointStatuses))
	for _, endpointStatus := range endpointStatuses {
		placeholders = append(placeholders, ui.Placeholder{Key: endpointStatus.Key, Name: endpointStatus.Name, Group: endpointStatus.Group})
	}
	cache.SetWithTTL("placeholders", placeholders, cacheTTL)
	return placeholders, nil
}

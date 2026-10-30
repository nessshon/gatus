package api

import (
	_ "embed"
	"html/template"

	"github.com/TwiN/gatus/v5/config"
	"github.com/TwiN/gatus/v5/config/ui"
	static "github.com/TwiN/gatus/v5/web"
	"github.com/TwiN/logr"
	"github.com/gofiber/fiber/v2"
)

func SinglePageApplication(cfg *config.Config) fiber.Handler {
	uiConfig := cfg.UI
	hasOIDC := cfg.Security != nil && cfg.Security.OIDC != nil
	// Same endpoints main.initializeStorage keeps statuses for, i.e. what /api/v1/endpoints/statuses returns
	placeholderCount := len(cfg.Endpoints) + len(cfg.ExternalEndpoints)
	return func(c *fiber.Ctx) error {
		vd := ui.ViewData{UI: uiConfig, OIDC: hasOIDC, PlaceholderCount: placeholderCount}
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

package api

import (
	"encoding/json"

	"github.com/TwiN/gatus/v5/config/ui"
	"github.com/gofiber/fiber/v2"
)

type manifestIcon struct {
	Src   string `json:"src"`
	Sizes string `json:"sizes"`
	Type  string `json:"type"`
}

// Upstream's manifest.json fields, branding from ui
type manifest struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Description     string         `json:"description"`
	Lang            string         `json:"lang"`
	Scope           string         `json:"scope"`
	StartURL        string         `json:"start_url"`
	ThemeColor      string         `json:"theme_color"`
	BackgroundColor string         `json:"background_color"`
	Display         string         `json:"display"`
	Icons           []manifestIcon `json:"icons"`
}

type ManifestHandler struct {
	manifest []byte
}

func NewManifestHandler(cfg *ui.Config) ManifestHandler {
	body, _ := json.Marshal(manifest{
		ID:              "gatus",
		Name:            cfg.Header,
		ShortName:       cfg.Header,
		Description:     cfg.Description,
		Lang:            "en",
		Scope:           "/",
		StartURL:        "/",
		ThemeColor:      "#f7f9fb",
		BackgroundColor: "#f7f9fb",
		Display:         "standalone",
		Icons: []manifestIcon{
			{Src: cfg.Favicon.Size192x192, Sizes: "192x192", Type: "image/png"},
			{Src: cfg.Favicon.Size512x512, Sizes: "512x512", Type: "image/png"},
		},
	})
	return ManifestHandler{manifest: body}
}

func (handler ManifestHandler) GetManifest(c *fiber.Ctx) error {
	c.Set("Content-Type", "application/manifest+json")
	return c.Status(200).Send(handler.manifest)
}

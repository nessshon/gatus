package ui

import (
	"bytes"
	"errors"
	"html/template"

	"github.com/TwiN/gatus/v5/storage"
	static "github.com/TwiN/gatus/v5/web"
)

const (
	defaultTitle               = "Health Dashboard | Gatus"
	defaultDescription         = "Gatus is an advanced automated status page that lets you monitor your applications and configure alerts to notify you if there's an issue"
	defaultHeader              = "Gatus"
	defaultDashboardHeading    = "Health Dashboard"
	defaultDashboardSubheading = "Monitor the health of your endpoints in real-time"
	defaultLogo                = ""
	defaultLink                = ""
	defaultFavicon             = "/favicon.ico"
	defaultFavicon16           = "/favicon-16x16.png"
	defaultFavicon32           = "/favicon-32x32.png"
	defaultFavicon180          = "/apple-touch-icon.png"
	defaultFavicon192          = "/logo-192x192.png"
	defaultFavicon512          = "/logo-512x512.png"
	defaultCustomCSS           = ""
	defaultSortBy              = "name"
	defaultFilterBy            = "none"
	defaultLoginSubtitle       = "System Monitoring Dashboard"
)

var (
	defaultDarkMode = true

	ErrButtonValidationFailed = errors.New("invalid button configuration: missing required name or link")
	ErrInvalidDefaultSortBy   = errors.New("invalid default-sort-by value: must be 'name', 'group', or 'health'")
	ErrInvalidDefaultFilterBy = errors.New("invalid default-filter-by value: must be 'none', 'failing', or 'unstable'")
)

// Config is the configuration for the UI of Gatus
type Config struct {
	Title                  string   `yaml:"title,omitempty"`                    // Title of the page
	Description            string   `yaml:"description,omitempty"`              // Meta description of the page
	DashboardHeading       string   `yaml:"dashboard-heading,omitempty"`        // Dashboard Title between header and endpoints
	DashboardSubheading    string   `yaml:"dashboard-subheading,omitempty"`     // Dashboard Description between header and endpoints
	Header                 string   `yaml:"header,omitempty"`                   // Header is the text at the top of the page
	Logo                   string   `yaml:"logo,omitempty"`                     // Logo to display on the page
	Link                   string   `yaml:"link,omitempty"`                     // Link to open when clicking on the logo
	Favicon                Favicon  `yaml:"favicon,omitempty"`                  // Favourite icon to display in web browser tab or address bar
	Buttons                []Button `yaml:"buttons,omitempty"`                  // Buttons to display below the header
	CustomCSS              string   `yaml:"custom-css,omitempty"`               // Custom CSS to include in the page
	DarkMode               *bool    `yaml:"dark-mode,omitempty"`                // DarkMode is a flag to enable dark mode by default
	DefaultSortBy          string   `yaml:"default-sort-by,omitempty"`          // DefaultSortBy is the default sort option ('name', 'group', 'health')
	DefaultFilterBy        string   `yaml:"default-filter-by,omitempty"`        // DefaultFilterBy is the default filter option ('none', 'failing', 'unstable')
	LoginSubtitle          string   `yaml:"login-subtitle,omitempty"`           // LoginSubtitle is the subtitle displayed on the OIDC login page
	Footer                 string   `yaml:"footer,omitempty"`                   // Footer text; empty shows the default "Powered by Gatus"
	Social                 string   `yaml:"social,omitempty"`                   // URL for the footer social icon; empty uses the default GitHub link
	HideControls           bool     `yaml:"hide-controls,omitempty"`            // HideControls hides the search/filter/sort bar
	DefaultExpanded        bool     `yaml:"default-expanded,omitempty"`         // DefaultExpanded expands all groups by default (until the visitor collapses one)
	DefaultRefreshInterval int      `yaml:"default-refresh-interval,omitempty"` // DefaultRefreshInterval is the initial UI auto-refresh in seconds (10/30/60/120/300/600); 0 keeps the built-in default
	//////////////////////////////////////////////
	// Non-configurable - used for UI rendering //
	//////////////////////////////////////////////
	MaximumNumberOfResults int `yaml:"-"` // MaximumNumberOfResults to display on the page, it's not configurable because we're passing it from the storage config
}

func (cfg *Config) IsDarkMode() bool {
	if cfg.DarkMode != nil {
		return *cfg.DarkMode
	}
	return defaultDarkMode
}

// Button is the configuration for a button on the UI
type Button struct {
	Name string `yaml:"name,omitempty"` // Name is the text to display on the button
	Link string `yaml:"link,omitempty"` // Link to open when the button is clicked.
}

// Validate validates the button configuration
func (btn *Button) Validate() error {
	if len(btn.Name) == 0 || len(btn.Link) == 0 {
		return ErrButtonValidationFailed
	}
	return nil
}

type Favicon struct {
	Default     string `yaml:"default,omitempty"`     // URL or path to default favourite icon.
	Size16x16   string `yaml:"size16x16,omitempty"`   // URL or path to favourite icon for 16x16 size.
	Size32x32   string `yaml:"size32x32,omitempty"`   // URL or path to favourite icon for 32x32 size.
	Size180x180 string `yaml:"size180x180,omitempty"` // URL or path to the apple-touch-icon (180x180).
	Size192x192 string `yaml:"size192x192,omitempty"` // URL or path to the 192x192 icon in the web app manifest.
	Size512x512 string `yaml:"size512x512,omitempty"` // URL or path to the 512x512 icon in the web app manifest.
}

// GetDefaultConfig returns a Config struct with the default values
func GetDefaultConfig() *Config {
	return &Config{
		Title:                  defaultTitle,
		Description:            defaultDescription,
		DashboardHeading:       defaultDashboardHeading,
		DashboardSubheading:    defaultDashboardSubheading,
		Header:                 defaultHeader,
		Logo:                   defaultLogo,
		Link:                   defaultLink,
		CustomCSS:              defaultCustomCSS,
		DarkMode:               &defaultDarkMode,
		DefaultSortBy:          defaultSortBy,
		DefaultFilterBy:        defaultFilterBy,
		LoginSubtitle:          defaultLoginSubtitle,
		MaximumNumberOfResults: storage.DefaultMaximumNumberOfResults,
		Favicon: Favicon{
			Default:     defaultFavicon,
			Size16x16:   defaultFavicon16,
			Size32x32:   defaultFavicon32,
			Size180x180: defaultFavicon180,
			Size192x192: defaultFavicon192,
			Size512x512: defaultFavicon512,
		},
	}
}

// ValidateAndSetDefaults validates the UI configuration and sets the default values if necessary.
func (cfg *Config) ValidateAndSetDefaults() error {
	if len(cfg.Title) == 0 {
		cfg.Title = defaultTitle
	}
	if len(cfg.Description) == 0 {
		cfg.Description = defaultDescription
	}
	if len(cfg.DashboardHeading) == 0 {
		cfg.DashboardHeading = defaultDashboardHeading
	}
	if len(cfg.DashboardSubheading) == 0 {
		cfg.DashboardSubheading = defaultDashboardSubheading
	}
	if len(cfg.Header) == 0 {
		cfg.Header = defaultHeader
	}
	if len(cfg.Logo) == 0 {
		cfg.Logo = defaultLogo
	}
	if len(cfg.Link) == 0 {
		cfg.Link = defaultLink
	}
	if len(cfg.CustomCSS) == 0 {
		cfg.CustomCSS = defaultCustomCSS
	}
	if cfg.DarkMode == nil {
		cfg.DarkMode = &defaultDarkMode
	}
	if len(cfg.DefaultSortBy) == 0 {
		cfg.DefaultSortBy = defaultSortBy
	} else if cfg.DefaultSortBy != "name" && cfg.DefaultSortBy != "group" && cfg.DefaultSortBy != "health" {
		return ErrInvalidDefaultSortBy
	}
	if len(cfg.DefaultFilterBy) == 0 {
		cfg.DefaultFilterBy = defaultFilterBy
	} else if cfg.DefaultFilterBy != "none" && cfg.DefaultFilterBy != "failing" && cfg.DefaultFilterBy != "unstable" {
		return ErrInvalidDefaultFilterBy
	}
	if len(cfg.LoginSubtitle) == 0 {
		cfg.LoginSubtitle = defaultLoginSubtitle
	}
	if len(cfg.Favicon.Default) == 0 {
		cfg.Favicon.Default = defaultFavicon
	}
	if len(cfg.Favicon.Size16x16) == 0 {
		cfg.Favicon.Size16x16 = defaultFavicon16
	}
	if len(cfg.Favicon.Size32x32) == 0 {
		cfg.Favicon.Size32x32 = defaultFavicon32
	}
	if len(cfg.Favicon.Size180x180) == 0 {
		cfg.Favicon.Size180x180 = defaultFavicon180
	}
	if len(cfg.Favicon.Size192x192) == 0 {
		cfg.Favicon.Size192x192 = defaultFavicon192
	}
	if len(cfg.Favicon.Size512x512) == 0 {
		cfg.Favicon.Size512x512 = defaultFavicon512
	}
	for _, btn := range cfg.Buttons {
		if err := btn.Validate(); err != nil {
			return err
		}
	}
	// Validate that the template works
	t, err := template.ParseFS(static.FileSystem, static.IndexPath)
	if err != nil {
		return err
	}
	var buffer bytes.Buffer
	return t.Execute(&buffer, ViewData{UI: cfg, Theme: "dark"})
}

type ViewData struct {
	UI    *Config
	Theme string
	// The page waits for /api/v1/config only with OIDC
	OIDC             bool
	PlaceholderCount int
	// Only without security, where /api/v1/endpoints/statuses already exposes names and groups
	Placeholders []Placeholder
}

type Placeholder struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Group string `json:"group,omitempty"`
}

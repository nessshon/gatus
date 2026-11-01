package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TwiN/gatus/v5/config"
	"github.com/TwiN/gatus/v5/config/endpoint"
	"github.com/TwiN/gatus/v5/config/ui"
	"github.com/TwiN/gatus/v5/security"
	"github.com/TwiN/gatus/v5/storage/store"
	"github.com/TwiN/gatus/v5/watchdog"
)

func TestSinglePageApplication(t *testing.T) {
	defer store.Get().Clear()
	defer cache.Clear()
	cfg := &config.Config{
		Metrics: true,
		Endpoints: []*endpoint.Endpoint{
			{
				Name:  "frontend",
				Group: "core",
			},
			{
				Name:  "backend",
				Group: "core",
			},
		},
		UI: &ui.Config{
			Title: "example-title",
		},
	}
	watchdog.UpdateEndpointStatus(cfg.Endpoints[0], &endpoint.Result{Success: true, Duration: time.Millisecond, Timestamp: time.Now()})
	watchdog.UpdateEndpointStatus(cfg.Endpoints[1], &endpoint.Result{Success: false, Duration: time.Second, Timestamp: time.Now()})
	api := New(cfg)
	router := api.Router()
	type Scenario struct {
		Name              string
		Path              string
		Gzip              bool
		CookieDarkMode    bool
		UIDarkMode        bool
		ExpectedCode      int
		ExpectedDarkTheme bool
	}
	scenarios := []Scenario{
		{
			Name:              "frontend-home",
			Path:              "/",
			CookieDarkMode:    true,
			UIDarkMode:        false,
			ExpectedDarkTheme: true,
			ExpectedCode:      200,
		},
		{
			Name:              "frontend-endpoint-light",
			Path:              "/endpoints/core_frontend",
			CookieDarkMode:    false,
			UIDarkMode:        false,
			ExpectedDarkTheme: false,
			ExpectedCode:      200,
		},
		{
			Name:              "frontend-endpoint-dark",
			Path:              "/endpoints/core_frontend",
			CookieDarkMode:    false,
			UIDarkMode:        true,
			ExpectedDarkTheme: true,
			ExpectedCode:      200,
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			cfg.UI.DarkMode = &scenario.UIDarkMode
			request := httptest.NewRequest("GET", scenario.Path, http.NoBody)
			if scenario.Gzip {
				request.Header.Set("Accept-Encoding", "gzip")
			}
			if scenario.CookieDarkMode {
				request.Header.Set("Cookie", "theme=dark")
			}
			response, err := router.Test(request)
			if err != nil {
				return
			}
			defer response.Body.Close()
			if response.StatusCode != scenario.ExpectedCode {
				t.Errorf("%s %s should have returned %d, but returned %d instead", request.Method, request.URL, scenario.ExpectedCode, response.StatusCode)
			}
			body, _ := io.ReadAll(response.Body)
			strBody := string(body)
			if !strings.Contains(strBody, cfg.UI.Title) {
				t.Errorf("%s %s should have contained the title", request.Method, request.URL)
			}
			if scenario.ExpectedDarkTheme && !strings.Contains(strBody, "class=\"dark\"") {
				t.Errorf("%s %s should have responded with dark mode headers", request.Method, request.URL)
			}
			if !scenario.ExpectedDarkTheme && strings.Contains(strBody, "class=\"dark\"") {
				t.Errorf("%s %s should not have responded with dark mode headers", request.Method, request.URL)
			}
		})
	}
}

func TestSinglePageApplication_Placeholders(t *testing.T) {
	protected := &security.Config{Basic: &security.BasicConfig{
		Username:                        "john.doe",
		PasswordBcryptHashBase64Encoded: "JDJhJDA4JDFoRnpPY1hnaFl1OC9ISlFsa21VS09wOGlPU1ZOTDlHZG1qeTFvb3dIckRBUnlHUmNIRWlT",
	}}
	for _, securityConfig := range []*security.Config{nil, protected} {
		cfg := &config.Config{
			Endpoints: []*endpoint.Endpoint{
				{Name: "frontend", Group: "core"},
				{Name: "backend"},
			},
			ExternalEndpoints: []*endpoint.ExternalEndpoint{
				{Name: "heartbeat", Group: "jobs"},
			},
			Security: securityConfig,
			UI:       ui.GetDefaultConfig(),
		}
		// Nothing was pushed to the external endpoint, so it has no status yet
		watchdog.UpdateEndpointStatus(cfg.Endpoints[0], &endpoint.Result{Success: true, Timestamp: time.Now()})
		watchdog.UpdateEndpointStatus(cfg.Endpoints[1], &endpoint.Result{Success: true, Timestamp: time.Now()})
		response, err := New(cfg).Router().Test(httptest.NewRequest("GET", "/", http.NoBody))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		store.Get().Clear()
		cache.Clear()
		strBody := string(body)
		// html/template pads numbers it writes into a script with spaces
		if !strings.Contains(strBody, "placeholders:  2 ,") {
			t.Errorf("expected 2 placeholders (protected=%v)", securityConfig != nil)
		}
		if strings.Contains(strBody, "heartbeat") || strings.Contains(strBody, "jobs") {
			t.Errorf("page must not contain an endpoint without a status (protected=%v)", securityConfig != nil)
		}
		if securityConfig == nil {
			if !strings.Contains(strBody, `placeholderLayout: [{"key":"_backend","name":"backend"},{"key":"core_frontend","name":"frontend","group":"core"}]}`) {
				t.Error("expected the placeholder layout in the same order as /api/v1/endpoints/statuses")
			}
			continue
		}
		// With security the page must not leak names or groups
		if !strings.Contains(strBody, "placeholderLayout:  null }") {
			t.Error("expected no placeholder layout with security")
		}
		for _, leaked := range []string{"frontend", "backend", "core"} {
			if strings.Contains(strBody, leaked) {
				t.Errorf("page must not contain %q with security", leaked)
			}
		}
	}
}

package telegram

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/TwiN/gatus/v5/alerting/alert"
	"github.com/TwiN/gatus/v5/client"
	"github.com/TwiN/gatus/v5/config/endpoint"
	"github.com/TwiN/gatus/v5/test"
)

func TestAlertProvider_Validate(t *testing.T) {
	t.Run("invalid-provider", func(t *testing.T) {
		invalidProvider := AlertProvider{DefaultConfig: Config{Token: "", ID: ""}}
		if err := invalidProvider.Validate(); err == nil {
			t.Error("provider shouldn't have been valid")
		}
	})
	t.Run("valid-provider", func(t *testing.T) {
		validProvider := AlertProvider{DefaultConfig: Config{Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", ID: "12345678"}}
		if err := validProvider.Validate(); err != nil {
			t.Error("provider should've been valid")
		}
	})
	t.Run("invalid-provider-override-nonexist-group", func(t *testing.T) {
		invalidProvider := AlertProvider{DefaultConfig: Config{Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", ID: "12345678"}, Overrides: []*Override{{Config: Config{Token: "token", ID: "id"}}}}
		if err := invalidProvider.Validate(); err == nil {
			t.Error("provider shouldn't have been valid")
		}
	})
	t.Run("invalid-provider-override-duplicate-group", func(t *testing.T) {
		invalidProvider := AlertProvider{DefaultConfig: Config{Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", ID: "12345678"}, Overrides: []*Override{{Group: "group1", Config: Config{Token: "token", ID: "id"}}, {Group: "group1", Config: Config{ID: "id2"}}}}
		if err := invalidProvider.Validate(); err == nil {
			t.Error("provider shouldn't have been valid")
		}
	})
	t.Run("valid-provider-with-overrides", func(t *testing.T) {
		validProvider := AlertProvider{DefaultConfig: Config{Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", ID: "12345678"}, Overrides: []*Override{{Group: "group", Config: Config{Token: "token", ID: "id"}}}}
		if err := validProvider.Validate(); err != nil {
			t.Error("provider should've been valid")
		}
	})
}

func TestAlertProvider_Send(t *testing.T) {
	defer client.InjectHTTPClient(nil)
	firstDescription := "description-1"
	secondDescription := "description-2"
	scenarios := []struct {
		Name             string
		Provider         AlertProvider
		Alert            alert.Alert
		Resolved         bool
		MockRoundTripper test.MockRoundTripper
		ExpectedError    bool
	}{
		{
			Name:     "triggered",
			Provider: AlertProvider{DefaultConfig: Config{ID: "123", Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"}},
			Alert:    alert.Alert{Description: &firstDescription, SuccessThreshold: 5, FailureThreshold: 3},
			Resolved: false,
			MockRoundTripper: test.MockRoundTripper(func(r *http.Request) *http.Response {
				return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
			}),
			ExpectedError: false,
		},
		{
			Name:     "triggered-error",
			Provider: AlertProvider{DefaultConfig: Config{ID: "123", Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"}},
			Alert:    alert.Alert{Description: &firstDescription, SuccessThreshold: 5, FailureThreshold: 3},
			Resolved: false,
			MockRoundTripper: test.MockRoundTripper(func(r *http.Request) *http.Response {
				// 400 isn't retried, a 5xx would wait out the ~90s backoff
				return &http.Response{StatusCode: http.StatusBadRequest, Body: http.NoBody}
			}),
			ExpectedError: true,
		},
		{
			Name:     "resolved",
			Provider: AlertProvider{DefaultConfig: Config{ID: "123", Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"}},
			Alert:    alert.Alert{Description: &secondDescription, SuccessThreshold: 5, FailureThreshold: 3},
			Resolved: true,
			MockRoundTripper: test.MockRoundTripper(func(r *http.Request) *http.Response {
				return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
			}),
			ExpectedError: false,
		},
		{
			Name:     "resolved-error",
			Provider: AlertProvider{DefaultConfig: Config{ID: "123", Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"}},
			Alert:    alert.Alert{Description: &secondDescription, SuccessThreshold: 5, FailureThreshold: 3},
			Resolved: true,
			MockRoundTripper: test.MockRoundTripper(func(r *http.Request) *http.Response {
				// 400 isn't retried, a 5xx would wait out the ~90s backoff
				return &http.Response{StatusCode: http.StatusBadRequest, Body: http.NoBody}
			}),
			ExpectedError: true,
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			client.InjectHTTPClient(&http.Client{Transport: scenario.MockRoundTripper})
			err := scenario.Provider.Send(
				&endpoint.Endpoint{Name: "endpoint-name"},
				&scenario.Alert,
				&endpoint.Result{
					ConditionResults: []*endpoint.ConditionResult{
						{Condition: "[CONNECTED] == true", Success: scenario.Resolved},
						{Condition: "[STATUS] == 200", Success: scenario.Resolved},
					},
				},
				scenario.Resolved,
			)
			if scenario.ExpectedError && err == nil {
				t.Error("expected error, got none")
			}
			if !scenario.ExpectedError && err != nil {
				t.Error("expected no error, got", err.Error())
			}
		})
	}
}

func TestAlertProvider_buildMessageText(t *testing.T) {
	scenarios := []struct {
		Name         string
		Config       Config
		Endpoint     endpoint.Endpoint
		Alert        alert.Alert
		Conditions   []*endpoint.ConditionResult
		Errors       []string
		Resolved     bool
		ExpectedText string
	}{
		{
			Name:     "triggered: linked name, bold group, healthcheck line, bulleted failed conditions",
			Config:   Config{ID: "123"},
			Endpoint: endpoint.Endpoint{Name: "backend", Group: "core", URL: "https://example.org/health"},
			Alert:    alert.Alert{FailureThreshold: 3, SuccessThreshold: 2},
			Conditions: []*endpoint.ConditionResult{
				{Condition: "[STATUS] (503) == 200", Success: false},
				{Condition: "[BODY].status (down) == ok", Success: false},
			},
			Resolved:     false,
			ExpectedText: "🔴 <b>core</b> / <a href=\"https://example.org/health\">backend</a>\n\n<i>healthcheck failed 3 time(s) in a row</i>\n\n<b>Condition results:</b>\n• <code>[STATUS] (503) == 200</code>\n• <code>[BODY].status (down) == ok</code>",
		},
		{
			Name:         "resolved is short: no conditions block",
			Config:       Config{ID: "123"},
			Endpoint:     endpoint.Endpoint{Name: "backend", Group: "core", URL: "https://example.org/health"},
			Alert:        alert.Alert{FailureThreshold: 3, SuccessThreshold: 2},
			Conditions:   []*endpoint.ConditionResult{{Condition: "[STATUS] (200) == 200", Success: true}},
			Resolved:     true,
			ExpectedText: "🟢 <b>core</b> / <a href=\"https://example.org/health\">backend</a>\n\n<i>healthcheck passing 2 time(s) in a row</i>",
		},
		{
			Name:         "custom-emoji on wraps premium tag",
			Config:       Config{ID: "123", CustomEmoji: true},
			Endpoint:     endpoint.Endpoint{Name: "backend", Group: "core", URL: "https://example.org/health"},
			Alert:        alert.Alert{FailureThreshold: 1},
			Conditions:   []*endpoint.ConditionResult{{Condition: "[STATUS] (503) == 200", Success: false}},
			Resolved:     false,
			ExpectedText: "<tg-emoji emoji-id='5240431636713065543'>🔴</tg-emoji> <b>core</b> / <a href=\"https://example.org/health\">backend</a>\n\n<i>healthcheck failed 1 time(s) in a row</i>\n\n<b>Condition results:</b>\n• <code>[STATUS] (503) == 200</code>",
		},
		{
			Name:         "no url: name is bold, not a link",
			Config:       Config{ID: "123"},
			Endpoint:     endpoint.Endpoint{Name: "demo-bot", Group: "bots"},
			Alert:        alert.Alert{FailureThreshold: 2},
			Conditions:   []*endpoint.ConditionResult{{Condition: "[CONNECTED] (false) == true", Success: false}},
			Resolved:     false,
			ExpectedText: "🔴 <b>bots</b> / <b>demo-bot</b>\n\n<i>healthcheck failed 2 time(s) in a row</i>\n\n<b>Condition results:</b>\n• <code>[CONNECTED] (false) == true</code>",
		},
		{
			Name:     "html-unsafe condition + url are escaped so the alert can't be dropped",
			Config:   Config{ID: "123"},
			Endpoint: endpoint.Endpoint{Name: "frontend", Group: "core", URL: "https://example.org/?a=1&b=2"},
			Alert:    alert.Alert{FailureThreshold: 3},
			Conditions: []*endpoint.ConditionResult{
				{Condition: "[BODY] == pat(*<title>x</title>*)", Success: false},
			},
			Resolved:     false,
			ExpectedText: "🔴 <b>core</b> / <a href=\"https://example.org/?a=1&amp;b=2\">frontend</a>\n\n<i>healthcheck failed 3 time(s) in a row</i>\n\n<b>Condition results:</b>\n• <code>[BODY] == pat(*&lt;title&gt;x&lt;/title&gt;*)</code>",
		},
		{
			Name:         "external heartbeat: no conditions, error block html-escaped and newline-flattened",
			Config:       Config{ID: "123"},
			Endpoint:     endpoint.Endpoint{Name: "demo-bot", Group: "bots"},
			Alert:        alert.Alert{FailureThreshold: 3},
			Errors:       []string{"heartbeat: no update received within 5m0s\n<reset> & <peer>"},
			Resolved:     false,
			ExpectedText: "🔴 <b>bots</b> / <b>demo-bot</b>\n\n<i>healthcheck failed 3 time(s) in a row</i>\n\n<b>Error:</b>\n<code>heartbeat: no update received within 5m0s &lt;reset&gt; &amp; &lt;peer&gt;</code>",
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			text := buildMessageText(&scenario.Config, &scenario.Endpoint, &scenario.Alert, &endpoint.Result{ConditionResults: scenario.Conditions, Errors: scenario.Errors}, scenario.Resolved)
			if text != scenario.ExpectedText {
				t.Errorf("expected:\n%q\ngot:\n%q", scenario.ExpectedText, text)
			}
			// must still be valid JSON
			body, err := json.Marshal(Body{ChatID: scenario.Config.ID, Text: text, ParseMode: "HTML"})
			if err != nil {
				t.Error("expected valid JSON body, got error:", err.Error())
			}
			out := make(map[string]interface{})
			if err := json.Unmarshal(body, &out); err != nil {
				t.Error("expected body to be valid JSON, got error:", err.Error())
			}
		})
	}
}

func TestAlertProvider_Send_TokenNotLeakedOnInvalidURL(t *testing.T) {
	t.Parallel()
	const secretToken = "999999:SECRET-TOKEN-MUST-NOT-LEAK"
	// A control character makes http.NewRequest fail with a *url.Error carrying the full URL.
	provider := AlertProvider{DefaultConfig: Config{
		Token:  secretToken,
		ID:     "12345678",
		ApiUrl: "http://example.org\x7f",
	}}
	description := "test"
	err := provider.Send(
		&endpoint.Endpoint{Name: "endpoint-name"},
		&alert.Alert{Description: &description, SuccessThreshold: 1, FailureThreshold: 1},
		&endpoint.Result{ConditionResults: []*endpoint.ConditionResult{}},
		false,
	)
	if err == nil {
		t.Fatal("expected an error due to the invalid URL, got none")
	}
	if strings.Contains(err.Error(), secretToken) {
		t.Errorf("error message must not contain the bot token, got: %s", err.Error())
	}
}

func TestAlertProvider_GetDefaultAlert(t *testing.T) {
	if (&AlertProvider{DefaultAlert: &alert.Alert{}}).GetDefaultAlert() == nil {
		t.Error("expected default alert to be not nil")
	}
	if (&AlertProvider{DefaultAlert: nil}).GetDefaultAlert() != nil {
		t.Error("expected default alert to be nil")
	}
}

func TestAlertProvider_GetConfig(t *testing.T) {
	t.Run("get-token-with-override", func(t *testing.T) {
		provider := AlertProvider{DefaultConfig: Config{Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", ID: "12345678"}, Overrides: []*Override{{Group: "group", Config: Config{Token: "groupToken", ID: "overrideID"}}}}
		cfg, err := provider.GetConfig("group", &alert.Alert{})
		if err != nil {
			t.Error("expected no error, got", err)
		}
		if cfg.Token != "groupToken" {
			t.Error("token should have been 'groupToken'")
		}
		if cfg.ID != "overrideID" {
			t.Error("id should have been 'overrideID'")
		}
	})
	t.Run("get-default-token-with-overridden-id", func(t *testing.T) {
		provider := AlertProvider{DefaultConfig: Config{Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", ID: "12345678"}, Overrides: []*Override{{Group: "group", Config: Config{ID: "overrideID"}}}}
		cfg, err := provider.GetConfig("group", &alert.Alert{})
		if err != nil {
			t.Error("expected no error, got", err)
		}
		if cfg.Token != provider.DefaultConfig.Token {
			t.Error("token should have been the default token")
		}
		if cfg.ID != "overrideID" {
			t.Error("id should have been 'overrideID'")
		}
	})
	t.Run("get-default-token-with-overridden-token", func(t *testing.T) {
		provider := AlertProvider{DefaultConfig: Config{Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", ID: "12345678"}, Overrides: []*Override{{Group: "group", Config: Config{Token: "groupToken"}}}}
		cfg, err := provider.GetConfig("group", &alert.Alert{})
		if err != nil {
			t.Error("expected no error, got", err)
		}
		if cfg.Token != "groupToken" {
			t.Error("token should have been 'groupToken'")
		}
		if cfg.ID != provider.DefaultConfig.ID {
			t.Error("id should have been the default id")
		}
	})
	t.Run("get-default-token-with-overridden-token-and-alert-token-override", func(t *testing.T) {
		provider := AlertProvider{DefaultConfig: Config{Token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", ID: "12345678"}, Overrides: []*Override{{Group: "group", Config: Config{Token: "groupToken"}}}}
		alert := &alert.Alert{ProviderOverride: map[string]any{"token": "alertToken"}}
		cfg, err := provider.GetConfig("group", alert)
		if err != nil {
			t.Error("expected no error, got", err)
		}
		if cfg.Token != "alertToken" {
			t.Error("token should have been 'alertToken'")
		}
		if cfg.ID != provider.DefaultConfig.ID {
			t.Error("id should have been the default id")
		}
		// Test ValidateOverrides as well, since it really just calls GetConfig
		if err = provider.ValidateOverrides("group", alert); err != nil {
			t.Errorf("unexpected error: %s", err)
		}
	})
}

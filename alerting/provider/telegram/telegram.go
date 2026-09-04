package telegram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TwiN/gatus/v5/alerting/alert"
	"github.com/TwiN/gatus/v5/client"
	"github.com/TwiN/gatus/v5/config/endpoint"
	"github.com/TwiN/logr"
	"gopkg.in/yaml.v3"
)

const ApiURL = "https://api.telegram.org"

// Default premium emoji ids for custom-emoji mode; bots without access fall back to the plain 🔴/🟢
const (
	defaultTriggeredEmojiID = "5240431636713065543"
	defaultResolvedEmojiID  = "5240378276039379068"
)

// Telegram allows ~30 msg/s per bot: we serialize + throttle, honor 429 retry_after, back off on network/5xx
const (
	sendInterval = time.Second / 30
	maxRetries   = 7
	maxBackoff   = 30 * time.Second
)

// errForbidden = chat blocked the bot (403): not retried or bubbled up, just skip that recipient
var errForbidden = errors.New("telegram: chat forbade the bot")

var (
	ErrTokenNotSet            = errors.New("token not set")
	ErrIDNotSet               = errors.New("id not set")
	ErrDuplicateGroupOverride = errors.New("duplicate group override")
)

type Config struct {
	Token   string `yaml:"token"`
	ID      string `yaml:"id"`
	TopicID string `yaml:"topic-id,omitempty"`
	ApiUrl  string `yaml:"api-url"`

	// CustomEmoji enables premium <tg-emoji> icons; default false uses plain 🔴/🟢
	CustomEmoji bool `yaml:"custom-emoji,omitempty"`
	// TriggeredEmojiID / ResolvedEmojiID override the premium ids (only with CustomEmoji)
	TriggeredEmojiID string `yaml:"triggered-emoji-id,omitempty"`
	ResolvedEmojiID  string `yaml:"resolved-emoji-id,omitempty"`

	ClientConfig *client.Config `yaml:"client,omitempty"`
}

func (cfg *Config) Validate() error {
	if len(cfg.ApiUrl) == 0 {
		cfg.ApiUrl = ApiURL
	}
	if len(cfg.Token) == 0 {
		return ErrTokenNotSet
	}
	if len(cfg.ID) == 0 {
		return ErrIDNotSet
	}
	return nil
}

func (cfg *Config) Merge(override *Config) {
	if override.ClientConfig != nil {
		cfg.ClientConfig = override.ClientConfig
	}
	if len(override.Token) > 0 {
		cfg.Token = override.Token
	}
	if len(override.ID) > 0 {
		cfg.ID = override.ID
	}
	if len(override.TopicID) > 0 {
		cfg.TopicID = override.TopicID
	}
	if len(override.ApiUrl) > 0 {
		cfg.ApiUrl = override.ApiUrl
	}
	if override.CustomEmoji {
		cfg.CustomEmoji = true
	}
	if len(override.TriggeredEmojiID) > 0 {
		cfg.TriggeredEmojiID = override.TriggeredEmojiID
	}
	if len(override.ResolvedEmojiID) > 0 {
		cfg.ResolvedEmojiID = override.ResolvedEmojiID
	}
}

// AlertProvider is the configuration necessary for sending an alert using Telegram
type AlertProvider struct {
	DefaultConfig Config `yaml:",inline"`

	// DefaultAlert is the default alert configuration to use for endpoints with an alert of the appropriate type
	DefaultAlert *alert.Alert `yaml:"default-alert,omitempty"`

	// Overrides is a list of overrides that may be prioritized over the default configuration
	Overrides []*Override `yaml:"overrides,omitempty"`
}

// Override is a configuration that may be prioritized over the default configuration
type Override struct {
	Group  string `yaml:"group"`
	Config `yaml:",inline"`
}

// Validate the provider's configuration
func (provider *AlertProvider) Validate() error {
	registeredGroups := make(map[string]bool)
	if provider.Overrides != nil {
		for _, override := range provider.Overrides {
			if isAlreadyRegistered := registeredGroups[override.Group]; isAlreadyRegistered || override.Group == "" {
				return ErrDuplicateGroupOverride
			}
			registeredGroups[override.Group] = true
		}
	}
	return provider.DefaultConfig.Validate()
}

// Send delivers the alert to every comma-separated id in cfg.ID (throttled, retried).
// Returns an error only if every recipient failed for a non-forbidden reason.
func (provider *AlertProvider) Send(ep *endpoint.Endpoint, alert *alert.Alert, result *endpoint.Result, resolved bool) error {
	cfg, err := provider.GetConfig(ep.Group, alert)
	if err != nil {
		return err
	}
	text := buildMessageText(cfg, ep, alert, result, resolved)
	httpClient := client.GetHTTPClient(cfg.ClientConfig)
	url := fmt.Sprintf("%s/bot%s/sendMessage", cfg.ApiUrl, cfg.Token)
	ids := splitIDs(cfg.ID)
	var lastErr error
	anyDelivered := false
	for i, chatID := range ids {
		bodyAsJSON, _ := json.Marshal(Body{
			ChatID:                chatID,
			Text:                  text,
			ParseMode:             "HTML",
			DisableWebPagePreview: true,
			TopicID:               cfg.TopicID,
		})
		if sendErr := sendToChat(httpClient, url, chatID, bodyAsJSON); sendErr != nil {
			if !errors.Is(sendErr, errForbidden) {
				lastErr = sendErr
			}
			logr.Warnf("[telegram.Send] chat=%s: %s", chatID, sendErr.Error())
		} else {
			anyDelivered = true
		}
		if i < len(ids)-1 {
			time.Sleep(sendInterval)
		}
	}
	if anyDelivered {
		return nil
	}
	return lastErr // nil if every recipient was merely forbidden
}

// sendToChat posts one message, honoring Telegram's rate limits: retry_after on
// 429, exponential backoff on network/5xx, skip on 403, give up on other 4xx.
func sendToChat(httpClient *http.Client, url, chatID string, bodyAsJSON []byte) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyAsJSON))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := httpClient.Do(request)
		if err != nil { // network error — back off and retry
			time.Sleep(backoff(attempt))
			continue
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode < 400 {
			return nil
		}
		switch {
		case response.StatusCode == http.StatusTooManyRequests:
			time.Sleep(retryAfter(body))
		case response.StatusCode == http.StatusForbidden:
			return errForbidden
		case response.StatusCode >= 500:
			time.Sleep(backoff(attempt))
		default: // 400 and other 4xx are not transient — don't retry
			return fmt.Errorf("call to provider alert returned status code %d: %s", response.StatusCode, string(body))
		}
	}
	return fmt.Errorf("call to provider alert for chat %s exhausted %d retries", chatID, maxRetries)
}

// backoff returns 2^attempt seconds, capped at maxBackoff.
func backoff(attempt int) time.Duration {
	return min(time.Duration(1<<attempt)*time.Second, maxBackoff)
}

// retryAfter parses {"parameters":{"retry_after":N}} from a 429 body.
func retryAfter(body []byte) time.Duration {
	var parsed struct {
		Parameters struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Parameters.RetryAfter > 0 {
		return min(time.Duration(parsed.Parameters.RetryAfter)*time.Second, maxBackoff)
	}
	return time.Second
}

func splitIDs(raw string) []string {
	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			ids = append(ids, p)
		}
	}
	return ids
}

type Body struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview"`
	TopicID               string `json:"message_thread_id,omitempty"`
}

// buildMessageText renders the HTML message. Every dynamic value is html-escaped
// so a `<`, `>` or `&` in a condition/URL can't break parse_mode and drop the alert.
func buildMessageText(cfg *Config, ep *endpoint.Endpoint, alert *alert.Alert, result *endpoint.Result, resolved bool) string {
	var b strings.Builder
	b.WriteString(statusEmoji(cfg, resolved))
	b.WriteString(" ")
	if ep.Group != "" {
		b.WriteString("<b>")
		b.WriteString(html.EscapeString(ep.Group))
		b.WriteString("</b> / ")
	}
	if ep.URL != "" { // name links to the endpoint being checked
		b.WriteString(`<a href="`)
		b.WriteString(html.EscapeString(ep.URL))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(ep.Name))
		b.WriteString(`</a>`)
	} else {
		b.WriteString("<b>")
		b.WriteString(html.EscapeString(ep.Name))
		b.WriteString("</b>")
	}
	// healthcheck line (italic)
	b.WriteString("\n\n<i>")
	if resolved {
		b.WriteString(fmt.Sprintf("healthcheck passing %d time(s) in a row", alert.SuccessThreshold))
	} else {
		b.WriteString(fmt.Sprintf("healthcheck failed %d time(s) in a row", alert.FailureThreshold))
	}
	b.WriteString("</i>")
	// on a trigger, list only the failed conditions with their observed values
	if !resolved {
		var failed []*endpoint.ConditionResult
		for _, cr := range result.ConditionResults {
			if !cr.Success {
				failed = append(failed, cr)
			}
		}
		if len(failed) > 0 {
			b.WriteString("\n\n<b>Condition results:</b>")
			for _, cr := range failed {
				b.WriteString("\n• <code>")
				b.WriteString(html.EscapeString(cr.Condition))
				b.WriteString("</code>")
			}
		}
		// heartbeat/external endpoints have no conditions — the reason lands in
		// result.Errors; flatten newlines and escape so it can't break parse_mode
		if len(result.Errors) > 0 {
			b.WriteString("\n\n<b>Error:</b>")
			for _, e := range result.Errors {
				b.WriteString("\n<code>")
				b.WriteString(html.EscapeString(strings.ReplaceAll(e, "\n", " ")))
				b.WriteString("</code>")
			}
		}
	}
	return b.String()
}

// statusEmoji is the leading 🔴 (down) / 🟢 (up), wrapped in a premium
// <tg-emoji> tag when CustomEmoji is on.
func statusEmoji(cfg *Config, resolved bool) string {
	fallback, id := "🔴", emojiID(cfg.TriggeredEmojiID, defaultTriggeredEmojiID)
	if resolved {
		fallback, id = "🟢", emojiID(cfg.ResolvedEmojiID, defaultResolvedEmojiID)
	}
	if cfg.CustomEmoji {
		return fmt.Sprintf("<tg-emoji emoji-id='%s'>%s</tg-emoji>", id, fallback)
	}
	return fallback
}

func emojiID(configured, fallback string) string {
	if configured != "" {
		return configured
	}
	return fallback
}

// GetDefaultAlert returns the provider's default alert configuration
func (provider *AlertProvider) GetDefaultAlert() *alert.Alert {
	return provider.DefaultAlert
}

// GetConfig returns the configuration for the provider with the overrides applied
func (provider *AlertProvider) GetConfig(group string, alert *alert.Alert) (*Config, error) {
	cfg := provider.DefaultConfig
	// Handle group overrides
	if provider.Overrides != nil {
		for _, override := range provider.Overrides {
			if group == override.Group {
				cfg.Merge(&override.Config)
				break
			}
		}
	}
	// Handle alert overrides
	if len(alert.ProviderOverride) != 0 {
		overrideConfig := Config{}
		if err := yaml.Unmarshal(alert.ProviderOverrideAsBytes(), &overrideConfig); err != nil {
			return nil, err
		}
		cfg.Merge(&overrideConfig)
	}
	// Validate the configuration
	err := cfg.Validate()
	return &cfg, err
}

// ValidateOverrides validates the alert's provider override and, if present, the group override
func (provider *AlertProvider) ValidateOverrides(group string, alert *alert.Alert) error {
	_, err := provider.GetConfig(group, alert)
	return err
}

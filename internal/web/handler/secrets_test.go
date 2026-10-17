package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/django1982/ankerctl/internal/model"
)

func TestRedactSecrets(t *testing.T) {
	type nested struct {
		Token string `json:"token,omitempty"`
		Name  string `json:"name"`
	}
	type section struct {
		Key    string  `json:"key"`
		Token  string  `json:"token,omitempty"`
		Name   string  `json:"name"`
		Nested *nested `json:"nested,omitempty"`
	}

	tests := []struct {
		name           string
		in             section
		paths          []string
		wantEmpty      []string
		wantConfigured map[string]bool
		wantAbsent     []string
		wantKept       map[string]string
	}{
		{
			name:           "set secret is blanked and flagged",
			in:             section{Key: "s3cret", Name: "n"},
			paths:          []string{"key"},
			wantEmpty:      []string{"key"},
			wantConfigured: map[string]bool{"key_configured": true},
			wantKept:       map[string]string{"name": "n"},
		},
		{
			name:           "empty secret reports not configured",
			in:             section{},
			paths:          []string{"key"},
			wantEmpty:      []string{"key"},
			wantConfigured: map[string]bool{"key_configured": false},
		},
		{
			name:           "whitespace-only secret reports not configured",
			in:             section{Key: "   "},
			paths:          []string{"key"},
			wantEmpty:      []string{"key"},
			wantConfigured: map[string]bool{"key_configured": false},
		},
		{
			name:           "omitempty field missing from JSON still gets a flag",
			in:             section{},
			paths:          []string{"token"},
			wantEmpty:      []string{"token"},
			wantConfigured: map[string]bool{"token_configured": false},
		},
		{
			name:  "nested secret is redacted, siblings kept",
			in:    section{Nested: &nested{Token: "ha-token", Name: "cam"}},
			paths: []string{"nested.token"},
		},
		{
			name:       "missing nested parent is skipped",
			in:         section{Name: "n"},
			paths:      []string{"nested.token"},
			wantAbsent: []string{"nested"},
			wantKept:   map[string]string{"name": "n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := redactSecrets(tt.in, tt.paths...)
			if err != nil {
				t.Fatalf("redactSecrets: %v", err)
			}
			for _, k := range tt.wantEmpty {
				if v, ok := got[k]; !ok || v != "" {
					t.Errorf("%s = %#v, want empty string", k, v)
				}
			}
			for k, want := range tt.wantConfigured {
				if v, _ := got[k].(bool); v != want {
					t.Errorf("%s = %#v, want %v", k, got[k], want)
				}
			}
			for _, k := range tt.wantAbsent {
				if _, ok := got[k]; ok {
					t.Errorf("%s present, want absent", k)
				}
			}
			for k, want := range tt.wantKept {
				if v, _ := got[k].(string); v != want {
					t.Errorf("%s = %#v, want %q", k, got[k], want)
				}
			}
		})
	}

	t.Run("nested values", func(t *testing.T) {
		got, err := redactSecrets(section{Nested: &nested{Token: "ha-token", Name: "cam"}}, "nested.token")
		if err != nil {
			t.Fatalf("redactSecrets: %v", err)
		}
		n, ok := got["nested"].(map[string]any)
		if !ok {
			t.Fatalf("nested = %#v, want object", got["nested"])
		}
		if n["token"] != "" || n["token_configured"] != true || n["name"] != "cam" {
			t.Errorf("nested = %#v, want token blanked, token_configured=true, name kept", n)
		}
	})
}

func TestPayloadHasSecret(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		want    bool
	}{
		{"nil payload", nil, false},
		{"missing key", map[string]any{"other": "x"}, false},
		{"empty string", map[string]any{"token": ""}, false},
		{"whitespace", map[string]any{"token": "  "}, false},
		{"non-string", map[string]any{"token": 42}, false},
		{"value", map[string]any{"token": "abc"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := payloadHasSecret(tt.payload, "token"); got != tt.want {
				t.Errorf("payloadHasSecret = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSameEndpoint(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"http://ha.lan:8123", "http://ha.lan:8123", true},
		{"http://ha.lan:8123/", "http://ha.lan:8123", true},
		{" http://ha.lan:8123 ", "http://ha.lan:8123//", true},
		{"", "", true},
		{"http://ha.lan:8123", "http://evil.example", false},
		{"http://ha.lan:8123", "", false},
	}
	for _, tt := range tests {
		if got := sameEndpoint(tt.a, tt.b); got != tt.want {
			t.Errorf("sameEndpoint(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// secretsConfig returns a config with every write-only secret set to a value
// containing "LEAKME" so responses can be scanned for it.
func secretsConfig() *model.Config {
	cfg := &model.Config{
		Printers: []model.Printer{{SN: "SN-SEC", Name: "M5C", Model: "V8110"}},
		Camera:   model.DefaultCameraConfig(),
	}
	cfg.PrintMonitor = model.DefaultPrintMonitorConfig()
	cfg.PrintMonitor.OpenRouterKey = "LEAKME-openrouter"
	cfg.SmartSocket = model.DefaultSmartSocketConfig()
	cfg.SmartSocket.BaseURL = "http://ha.lan:8123"
	cfg.SmartSocket.Token = "LEAKME-socket"
	cfg.HomeAssistant.MQTTPassword = "LEAKME-mqtt"
	cfg.Notifications.Apprise.ServerURL = "http://apprise.lan:8000"
	cfg.Notifications.Apprise.Key = "LEAKME-apprise"
	cfg.Notifications.Announcement.BaseURL = "http://ha.lan:8123"
	cfg.Notifications.Announcement.Token = "LEAKME-announce"
	cfg.Camera.PerPrinter = map[string]model.PrinterCameraEntry{
		"SN-SEC": {
			Source: model.CameraSourceExternal,
			External: model.ExternalCameraSettings{
				RefreshSec: 3,
				HomeAssistant: &model.HomeAssistantCameraSettings{
					Enabled:        true,
					BaseURL:        "http://ha.lan:8123",
					Token:          "LEAKME-camera",
					CameraEntityID: "camera.printer",
				},
			},
		},
	}
	return cfg
}

func TestSettingsResponsesDoNotLeakSecrets(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		body    string
		handler func(*Handler) http.HandlerFunc
		flag    string
	}{
		{"print monitor get", http.MethodGet, "", func(h *Handler) http.HandlerFunc { return h.SettingsPrintMonitorGet }, `"openrouter_key_configured":true`},
		{"print monitor update", http.MethodPost, `{"print_monitor":{"interval_sec":120}}`, func(h *Handler) http.HandlerFunc { return h.SettingsPrintMonitorUpdate }, `"openrouter_key_configured":true`},
		{"smart socket get", http.MethodGet, "", func(h *Handler) http.HandlerFunc { return h.SettingsSmartSocketGet }, `"token_configured":true`},
		{"smart socket update", http.MethodPost, `{"smart_socket":{"token":""}}`, func(h *Handler) http.HandlerFunc { return h.SettingsSmartSocketUpdate }, `"token_configured":true`},
		{"mqtt get", http.MethodGet, "", func(h *Handler) http.HandlerFunc { return h.SettingsMQTTGet }, `"mqtt_password_configured":true`},
		{"mqtt update", http.MethodPost, `{"home_assistant":{"mqtt_port":1883}}`, func(h *Handler) http.HandlerFunc { return h.SettingsMQTTUpdate }, `"mqtt_password_configured":true`},
		{"notifications get", http.MethodGet, "", func(h *Handler) http.HandlerFunc { return h.NotificationsGet }, `"key_configured":true`},
		{"notifications update", http.MethodPost, `{"apprise":{"tag":"t"},"announcement":{"language":"de"}}`, func(h *Handler) http.HandlerFunc { return h.NotificationsUpdate }, `"token_configured":true`},
		{"camera get", http.MethodGet, "", func(h *Handler) http.HandlerFunc { return h.SettingsCameraGet }, `"token_configured":true`},
		{"camera update", http.MethodPost, `{"camera":{"external":{"refresh_sec":5}}}`, func(h *Handler) http.HandlerFunc { return h.SettingsCameraUpdate }, `"token_configured":true`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandlerWithConfig(t, secretsConfig())
			req := httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			tt.handler(h)(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if strings.Contains(body, "LEAKME") {
				t.Errorf("response leaks a stored secret: %s", body)
			}
			if !strings.Contains(body, tt.flag) {
				t.Errorf("response missing %s: %s", tt.flag, body)
			}
		})
	}

	// Updates that omit secrets (or send them empty) must keep the stored values.
	t.Run("updates keep stored secrets", func(t *testing.T) {
		h := newTestHandlerWithConfig(t, secretsConfig())
		for _, call := range []struct {
			fn   http.HandlerFunc
			body string
		}{
			{h.SettingsPrintMonitorUpdate, `{"print_monitor":{"openrouter_key":""}}`},
			{h.SettingsSmartSocketUpdate, `{"smart_socket":{"token":""}}`},
			{h.SettingsMQTTUpdate, `{"home_assistant":{"mqtt_password":""}}`},
			{h.NotificationsUpdate, `{"apprise":{"key":""},"announcement":{"token":""}}`},
			{h.SettingsCameraUpdate, `{"camera":{"external":{"home_assistant":{"token":""}}}}`},
		} {
			rec := httptest.NewRecorder()
			call.fn(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(call.body)))
			if rec.Code != http.StatusOK {
				t.Fatalf("update status = %d, body = %s", rec.Code, rec.Body.String())
			}
		}
		cfg, err := h.loadConfig()
		if err != nil {
			t.Fatalf("loadConfig: %v", err)
		}
		checks := map[string]string{
			"openrouter": cfg.PrintMonitor.OpenRouterKey,
			"socket":     cfg.SmartSocket.Token,
			"mqtt":       cfg.HomeAssistant.MQTTPassword,
			"apprise":    cfg.Notifications.Apprise.Key,
			"announce":   cfg.Notifications.Announcement.Token,
			"camera":     cfg.Camera.PerPrinter["SN-SEC"].External.HomeAssistant.Token,
		}
		for name, got := range checks {
			if got != "LEAKME-"+name {
				t.Errorf("%s secret = %q, want it kept", name, got)
			}
		}
	})
}

// recordingServer captures the Authorization headers it receives.
type recordingServer struct {
	*httptest.Server
	mu    sync.Mutex
	auths []string
}

func newRecordingServer(t *testing.T) *recordingServer {
	t.Helper()
	rs := &recordingServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.mu.Lock()
		rs.auths = append(rs.auths, r.Header.Get("Authorization"))
		rs.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(rs.Close)
	return rs
}

func (rs *recordingServer) seen() []string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]string(nil), rs.auths...)
}

func TestNotificationsTestDoesNotSendStoredTokenToNewURL(t *testing.T) {
	announcingConfig := func() *model.Config {
		cfg := secretsConfig()
		ann := &cfg.Notifications.Announcement
		ann.Enabled = true
		ann.TTSEntityID = "tts.speak"
		ann.MediaPlayerEntityID = "media_player.kitchen"
		ann.Template = "Test announcement"
		ann.Events.PrintFinished = true
		return cfg
	}

	t.Run("stored token is not sent to a caller-chosen URL", func(t *testing.T) {
		target := newRecordingServer(t)
		h := newTestHandlerWithConfig(t, announcingConfig())
		body := `{"announcement":{"base_url":"` + target.URL + `"}}`
		h.NotificationsTest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))

		for _, auth := range target.seen() {
			if strings.Contains(auth, "LEAKME") {
				t.Fatalf("stored token sent to overridden URL: %q", auth)
			}
		}
	})

	// Control: the same request path does reach the target when the caller
	// supplies the secret, so the test above is not passing vacuously.
	t.Run("explicit token is used for the new URL", func(t *testing.T) {
		target := newRecordingServer(t)
		h := newTestHandlerWithConfig(t, announcingConfig())
		body := `{"announcement":{"base_url":"` + target.URL + `","token":"explicit-token"}}`
		h.NotificationsTest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))

		seen := target.seen()
		if len(seen) == 0 {
			t.Fatal("target received no request; control path broken")
		}
		if seen[0] != "Bearer explicit-token" {
			t.Errorf("Authorization = %q, want Bearer explicit-token", seen[0])
		}
	})
}

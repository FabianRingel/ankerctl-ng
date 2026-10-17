package handler

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Secret field paths per settings section. Paths are dot-separated JSON keys
// relative to the section object returned by the API.
var (
	printMonitorSecretPaths = []string{"openrouter_key"}
	smartSocketSecretPaths  = []string{"token"}
	mqttSecretPaths         = []string{"mqtt_password"}
	appriseSecretPaths      = []string{"key"}
	announcementSecretPaths = []string{"token"}
	cameraSecretPaths       = []string{"external.home_assistant.token"}
)

// redactSecrets converts v to a JSON object and makes the given secret fields
// write-only: each existing field is blanked and a sibling
// "<field>_configured" boolean reports whether a value was stored. Settings
// responses must never echo stored credentials (Home Assistant tokens, API
// keys, passwords); the UI only needs to know whether one is saved, and the
// update handlers keep the stored value when the field is sent empty.
//
// Paths whose parent object is missing (e.g. no Home Assistant camera block)
// are skipped.
func redactSecrets(v any, paths ...string) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal settings: %w", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("settings are not a JSON object: %w", err)
	}
	if obj == nil {
		obj = map[string]any{}
	}
	for _, path := range paths {
		redactPath(obj, strings.Split(path, "."))
	}
	return obj, nil
}

func redactPath(obj map[string]any, segments []string) {
	for len(segments) > 1 {
		next, ok := obj[segments[0]].(map[string]any)
		if !ok {
			return
		}
		obj, segments = next, segments[1:]
	}
	leaf := segments[0]
	value, _ := obj[leaf].(string)
	obj[leaf] = ""
	obj[leaf+"_configured"] = strings.TrimSpace(value) != ""
}

// redactedSection is redactSecrets for handlers: it logs failures and returns
// ok=false so the caller can answer 500 instead of falling back to the raw
// (unredacted) section.
func (h *Handler) redactedSection(section any, paths ...string) (map[string]any, bool) {
	redacted, err := redactSecrets(section, paths...)
	if err != nil {
		h.log.Error("redact settings secrets", "err", err)
		return nil, false
	}
	return redacted, true
}

// payloadHasSecret reports whether a request payload carries a non-empty
// string value for key. A nil payload has none.
func payloadHasSecret(payload map[string]any, key string) bool {
	v, _ := payload[key].(string)
	return strings.TrimSpace(v) != ""
}

// sameEndpoint reports whether two configured base URLs refer to the same
// endpoint, ignoring surrounding whitespace and trailing slashes.
func sameEndpoint(a, b string) bool {
	norm := func(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }
	return norm(a) == norm(b)
}

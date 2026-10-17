package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// ParseTrustedOrigins parses a comma-separated list of origins
// ("scheme://host[:port]") that may make cross-origin requests, e.g. a
// dashboard served from another host. Empty entries are dropped and trailing
// slashes removed. Format errors surface when the list is applied.
func ParseTrustedOrigins(csv string) []string {
	var origins []string
	for _, item := range strings.Split(csv, ",") {
		o := strings.TrimRight(strings.TrimSpace(item), "/")
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

// CrossOriginProtection rejects state-changing browser requests (POST, PUT,
// PATCH, DELETE) that come from another origin, so a web page opened by
// someone on the LAN cannot send G-code or change settings (CSRF). It uses
// Sec-Fetch-Site and falls back to comparing Origin with Host. Requests
// without either header (OrcaSlicer, curl, Home Assistant) are not browser
// requests and pass; they authenticate with the API key.
func CrossOriginProtection(trusted []string, logger *slog.Logger) (func(http.Handler) http.Handler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	cop := http.NewCrossOriginProtection()
	for _, origin := range trusted {
		if err := cop.AddTrustedOrigin(origin); err != nil {
			return nil, fmt.Errorf("trusted origin %q: %w", origin, err)
		}
	}
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Warn("blocked cross-origin request",
			"method", r.Method,
			"path", r.URL.Path,
			"origin", r.Header.Get("Origin"),
			"sec_fetch_site", r.Header.Get("Sec-Fetch-Site"))
		writeJSONError(w, http.StatusForbidden, "Cross-origin request blocked")
	}))
	return cop.Handler, nil
}

// WebSocketOriginAllowed reports whether a WebSocket handshake may proceed.
// Browsers always send Origin on WebSocket handshakes and WebSockets are not
// covered by CORS, so without this check any web page could open the
// control and telemetry sockets. Handshakes without Origin come from
// non-browser clients and are allowed.
func WebSocketOriginAllowed(r *http.Request, trusted []string) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	if trustProxyEnabled() {
		if fwd := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0]); fwd != "" && strings.EqualFold(u.Host, fwd) {
			return true
		}
	}
	for _, t := range trusted {
		if strings.EqualFold(strings.TrimRight(origin, "/"), t) {
			return true
		}
	}
	return false
}

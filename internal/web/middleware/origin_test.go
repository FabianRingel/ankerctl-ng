package middleware

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestParseTrustedOrigins(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{" , ,", nil},
		{"https://ha.lan", []string{"https://ha.lan"}},
		{" https://ha.lan/ , http://10.0.0.5:8123 ", []string{"https://ha.lan", "http://10.0.0.5:8123"}},
	}
	for _, tt := range tests {
		if got := ParseTrustedOrigins(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseTrustedOrigins(%q) = %#v, want %#v", tt.in, got, tt.want)
		}
	}
}

func TestCrossOriginProtection(t *testing.T) {
	const host = "192.168.1.10:4470"
	tests := []struct {
		name          string
		method        string
		secFetchSite  string
		origin        string
		trusted       []string
		wantStatus    int
		wantReachNext bool
	}{
		{"safe method from other site", http.MethodGet, "cross-site", "https://evil.example", nil, http.StatusOK, true},
		{"non-browser client (OrcaSlicer, curl)", http.MethodPost, "", "", nil, http.StatusOK, true},
		{"same-origin fetch", http.MethodPost, "same-origin", "http://" + host, nil, http.StatusOK, true},
		{"user-initiated navigation", http.MethodPost, "none", "", nil, http.StatusOK, true},
		{"cross-site fetch", http.MethodPost, "cross-site", "https://evil.example", nil, http.StatusForbidden, false},
		{"same-site other port", http.MethodPost, "same-site", "http://192.168.1.10:8080", nil, http.StatusForbidden, false},
		{"cross-site delete", http.MethodDelete, "cross-site", "https://evil.example", nil, http.StatusForbidden, false},
		{"old browser, matching origin", http.MethodPost, "", "http://" + host, nil, http.StatusOK, true},
		{"old browser, foreign origin", http.MethodPost, "", "https://evil.example", nil, http.StatusForbidden, false},
		{"opaque null origin", http.MethodPost, "", "null", nil, http.StatusForbidden, false},
		{"trusted cross-site origin", http.MethodPost, "cross-site", "https://dash.lan", []string{"https://dash.lan"}, http.StatusOK, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mwFn, err := CrossOriginProtection(tt.trusted, nil)
			if err != nil {
				t.Fatalf("CrossOriginProtection: %v", err)
			}
			reached := false
			h := mwFn(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, "http://"+host+"/api/printer/gcode", nil)
			req.Host = host
			if tt.secFetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tt.secFetchSite)
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if reached != tt.wantReachNext {
				t.Errorf("next handler reached = %v, want %v", reached, tt.wantReachNext)
			}
		})
	}
}

func TestCrossOriginProtection_InvalidTrustedOrigin(t *testing.T) {
	if _, err := CrossOriginProtection([]string{"not a url/with/path"}, nil); err == nil {
		t.Fatal("expected error for malformed trusted origin")
	}
}

func TestWebSocketOriginAllowed(t *testing.T) {
	const host = "192.168.1.10:4470"
	tests := []struct {
		name       string
		origin     string
		fwdHost    string
		trustProxy bool
		trusted    []string
		want       bool
	}{
		{"no origin (non-browser client)", "", "", false, nil, true},
		{"same origin", "http://" + host, "", false, nil, true},
		{"same host, other port", "http://192.168.1.10:8080", "", false, nil, false},
		{"foreign site", "https://evil.example", "", false, nil, false},
		{"null origin", "null", "", false, nil, false},
		{"malformed origin", "::::", "", false, nil, false},
		{"trusted origin", "https://dash.lan", "", false, []string{"https://dash.lan"}, true},
		{"proxy host honoured when trusted", "https://printer.example", "printer.example", true, nil, true},
		{"proxy host ignored when not trusted", "https://printer.example", "printer.example", false, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.trustProxy {
				t.Setenv("ANKERCTL_TRUST_PROXY", "true")
			} else {
				t.Setenv("ANKERCTL_TRUST_PROXY", "")
			}
			req := httptest.NewRequest(http.MethodGet, "http://"+host+"/ws/ctrl", nil)
			req.Host = host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.fwdHost != "" {
				req.Header.Set("X-Forwarded-Host", tt.fwdHost)
			}
			if got := WebSocketOriginAllowed(req, tt.trusted); got != tt.want {
				t.Errorf("WebSocketOriginAllowed = %v, want %v", got, tt.want)
			}
		})
	}
}

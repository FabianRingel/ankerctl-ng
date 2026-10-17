package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

// originTestState adds an explicit origin policy to testState.
type originTestState struct {
	testState
	allow func(*http.Request) bool
}

func (s originTestState) WebSocketOriginAllowed(r *http.Request) bool { return s.allow(r) }

func TestWebSocketHandshakeOrigin(t *testing.T) {
	base := testState{loggedIn: true, probePPPP: func(context.Context) bool { return true }}
	trustedOnly := originTestState{
		testState: base,
		allow:     func(r *http.Request) bool { return r.Header.Get("Origin") == "https://dash.lan" },
	}

	tests := []struct {
		name   string
		st     state
		origin string // "self" = the test server's own origin
		wantOK bool
	}{
		{"default policy, no origin", base, "", true},
		{"default policy, same origin", base, "self", true},
		{"default policy, foreign origin", base, "https://evil.example", false},
		{"server policy, trusted origin", trustedOnly, "https://dash.lan", true},
		{"server policy, foreign origin", trustedOnly, "https://evil.example", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(nil, tt.st, nil)
			srv := httptest.NewServer(http.HandlerFunc(h.PPPPState))
			defer srv.Close()

			header := http.Header{}
			switch tt.origin {
			case "":
			case "self":
				header.Set("Origin", srv.URL)
			default:
				header.Set("Origin", tt.origin)
			}
			wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
			conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
			if conn != nil {
				_ = conn.Close()
			}
			if tt.wantOK && err != nil {
				t.Fatalf("handshake failed: %v", err)
			}
			if !tt.wantOK {
				if err == nil {
					t.Fatal("handshake succeeded, want rejection")
				}
				if resp == nil || resp.StatusCode != http.StatusForbidden {
					t.Fatalf("response = %v, want 403", resp)
				}
			}
		})
	}
}

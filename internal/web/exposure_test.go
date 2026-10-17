package web

import (
	"errors"
	"testing"

	"github.com/django1982/ankerctl/internal/config"
)

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"127.10.0.5", true},
		{"::1", true},
		{"[::1]", true},
		{"localhost", true},
		{"LOCALHOST", true},
		{"localhost.", true},
		{" 127.0.0.1 ", true},
		{"", false},
		{"0.0.0.0", false},
		{"::", false},
		{"[::]", false},
		{"192.168.1.20", false},
		{"10.43.0.1", false},
		{"printer.lan", false},
		{"fe80::1%eth0", false},
	}
	for _, tt := range tests {
		if got := IsLoopbackHost(tt.host); got != tt.want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestValidateExposure(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		apiKey        string
		allowInsecure bool
		wantErr       bool
	}{
		{"loopback without key", "127.0.0.1", "", false, false},
		{"localhost without key", "localhost", "", false, false},
		{"wildcard without key", "0.0.0.0", "", false, true},
		{"empty host without key", "", "", false, true},
		{"lan ip without key", "192.168.1.20", "", false, true},
		{"whitespace key counts as missing", "0.0.0.0", "   ", false, true},
		{"wildcard with key", "0.0.0.0", "k3y-0123456789abcdef", false, false},
		{"wildcard with explicit opt-out", "0.0.0.0", "", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExposure(tt.host, tt.apiKey, tt.allowInsecure)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateExposure() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInsecureExposure) {
				t.Errorf("error %v does not wrap ErrInsecureExposure", err)
			}
		})
	}
}

func TestServerInitialize_ExposureGuard(t *testing.T) {
	newServer := func(t *testing.T, opts ...Option) *Server {
		t.Helper()
		cfgMgr, err := config.NewManager(t.TempDir())
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}
		return NewServer(cfgMgr, opts...)
	}

	t.Run("non-loopback without key fails", func(t *testing.T) {
		t.Setenv("ANKERCTL_API_KEY", "")
		t.Setenv(allowInsecureEnv, "")
		s := newServer(t, WithHost("0.0.0.0"), WithPort(4470))
		if err := s.initialize(); !errors.Is(err, ErrInsecureExposure) {
			t.Fatalf("initialize() error = %v, want ErrInsecureExposure", err)
		}
	})

	t.Run("API key from env allows non-loopback", func(t *testing.T) {
		t.Setenv("ANKERCTL_API_KEY", "k3y-0123456789abcdef")
		t.Setenv(allowInsecureEnv, "")
		s := newServer(t, WithHost("0.0.0.0"), WithPort(4470))
		if err := s.initialize(); err != nil {
			t.Fatalf("initialize() error = %v", err)
		}
	})

	t.Run("env opt-out allows non-loopback", func(t *testing.T) {
		t.Setenv("ANKERCTL_API_KEY", "")
		t.Setenv(allowInsecureEnv, "true")
		s := newServer(t, WithHost("0.0.0.0"), WithPort(4470))
		if err := s.initialize(); err != nil {
			t.Fatalf("initialize() error = %v", err)
		}
	})

	t.Run("WithInsecure is an explicit opt-out", func(t *testing.T) {
		t.Setenv("ANKERCTL_API_KEY", "")
		t.Setenv(allowInsecureEnv, "")
		s := newServer(t, WithHost("0.0.0.0"), WithPort(4470), WithInsecure(true))
		if err := s.initialize(); err != nil {
			t.Fatalf("initialize() error = %v", err)
		}
	})

	t.Run("default loopback host needs no key", func(t *testing.T) {
		t.Setenv("ANKERCTL_API_KEY", "")
		t.Setenv(allowInsecureEnv, "")
		t.Setenv("ANKERCTL_HOST", "")
		t.Setenv("FLASK_HOST", "")
		s := newServer(t)
		if err := s.initialize(); err != nil {
			t.Fatalf("initialize() error = %v", err)
		}
	})
}

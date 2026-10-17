package web

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// allowInsecureEnv opts out of the exposure guard. Only meant for setups where
// another layer (firewall, authenticating reverse proxy) restricts access.
const allowInsecureEnv = "ANKERCTL_ALLOW_INSECURE"

// ErrInsecureExposure is returned when the web server would accept
// unauthenticated requests from other hosts.
var ErrInsecureExposure = errors.New("refusing to serve without an API key on a non-loopback address")

// ValidateExposure rejects listening on a non-loopback address without an API
// key. Without a key every endpoint is open, so anyone who can reach the port
// could send G-code, switch the smart socket and read stored settings.
func ValidateExposure(host, apiKey string, allowInsecure bool) error {
	if strings.TrimSpace(apiKey) != "" || allowInsecure || IsLoopbackHost(host) {
		return nil
	}
	where := host
	if strings.TrimSpace(where) == "" {
		where = "all interfaces"
	}
	return fmt.Errorf("%w (%s): anyone who can reach this port could control the printer. "+
		"Set ANKERCTL_API_KEY (or run `ankerctl-ng config api-key create`), bind to 127.0.0.1, "+
		"or set %s=true to accept the risk", ErrInsecureExposure, where, allowInsecureEnv)
}

// IsLoopbackHost reports whether binding to host only accepts local
// connections. Empty hosts and wildcards bind every interface; hostnames other
// than localhost are treated as non-loopback because they may resolve to a LAN
// address.
func IsLoopbackHost(host string) bool {
	h := strings.Trim(strings.TrimSpace(host), "[]")
	if h == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSuffix(h, "."), "localhost") {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// InsecureExposureAllowed reports whether ANKERCTL_ALLOW_INSECURE disables the
// exposure guard.
func InsecureExposureAllowed() bool {
	return envBool(allowInsecureEnv, false)
}

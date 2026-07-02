package gateway

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateGatewayURL rejects vault gateway URLs that are not HTTPS with a host,
// except for HTTP on loopback hosts used by local development and integration tests.
func ValidateGatewayURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid vault gateway URL: %w", err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("invalid vault gateway URL")
	}
	if u.Host == "" {
		return fmt.Errorf("vault gateway URL is missing a host")
	}

	host := u.Hostname()
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopbackHost(host) {
			return nil
		}
		return fmt.Errorf("vault gateway URL must use https://, got %q", u.Scheme)
	default:
		return fmt.Errorf("vault gateway URL must use https://, got %q", u.Scheme)
	}
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

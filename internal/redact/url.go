package redact

import (
	"fmt"
	"net/url"
	"strings"
)

// URL returns a version of rawURL with credentials masked to avoid leaking secrets
// that may be embedded in RPC or asset URLs.
// For example, "https://rpc.example.com/v1/my-secret-key" becomes "https://rpc.example.com/v1/***".
func URL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return RedactedValue
	}

	host := u.Host
	if u.User != nil {
		username := u.User.Username()
		if username != "" {
			host = username + ":***@" + u.Hostname()
			if port := u.Port(); port != "" {
				host += ":" + port
			}
		}
	}

	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path != "" && u.Path != "/" {
		parts := strings.Split(u.Path, "/")
		if len(parts) > 1 {
			parts[len(parts)-1] = "***"
		}
		u.RawPath = ""
		u.Path = strings.Join(parts, "/")
	}
	u.RawQuery = ""
	u.Fragment = ""
	return fmt.Sprintf("%s://%s%s", u.Scheme, host, u.Path)
}

package redact

import (
	"fmt"
	"net/url"
	"strings"
)

// URL returns a version of rawURL with the last path segment and query parameters
// masked to avoid leaking secrets that may be embedded in RPC or asset URLs.
func URL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return RedactedValue
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
	return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
}

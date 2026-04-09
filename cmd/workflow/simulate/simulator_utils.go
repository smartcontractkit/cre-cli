package simulate

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const WorkflowExecutionTimeout = 5 * time.Minute

type ChainSelector = uint64

// redactURL returns a version of the URL with path segments and query parameters
// masked to avoid leaking secrets that may have been resolved from environment variables.
// For example, "https://rpc.example.com/v1/my-secret-key" becomes "https://rpc.example.com/v1/***".
func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "***"
	}
	// Mask the last path segment (most common location for API keys)
	u.Path = strings.TrimRight(u.Path, "/")
	if u.Path != "" && u.Path != "/" {
		parts := strings.Split(u.Path, "/")
		if len(parts) > 1 {
			parts[len(parts)-1] = "***"
		}
		u.RawPath = ""
		u.Path = strings.Join(parts, "/")
	}
	// Remove query params entirely
	u.RawQuery = ""
	u.Fragment = ""
	// Use Opaque to avoid re-encoding the path
	return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
}

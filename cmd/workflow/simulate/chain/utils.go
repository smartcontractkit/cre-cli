package chain

import "github.com/smartcontractkit/cre-cli/internal/redact"

// RedactURL returns a version of the URL with path segments and query parameters
// masked to avoid leaking secrets that may have been resolved from environment variables.
// For example, "https://rpc.example.com/v1/my-secret-key" becomes "https://rpc.example.com/v1/***".
func RedactURL(rawURL string) string {
	return redact.URL(rawURL)
}

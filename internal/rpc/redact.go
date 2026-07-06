package rpc

import "github.com/smartcontractkit/cre-cli/internal/redact"

// RedactURL returns a version of the URL with credentials masked to avoid leaking
// secrets that may have been resolved from environment variables.
func RedactURL(rawURL string) string {
	return redact.URL(rawURL)
}

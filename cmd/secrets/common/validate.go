package common

import (
	"fmt"
	"strings"
)

const (
	SecretsAuthOwnerKeySigning = "owner-key-signing"
	SecretsAuthBrowser         = "browser"
)

// ValidateSecretsAuthFlow checks that the chosen auth flow is valid and
// allowed in the current environment. Browser flow is blocked in production.
func ValidateSecretsAuthFlow(flow, envName string) error {
	switch flow {
	case SecretsAuthOwnerKeySigning:
		return nil
	case SecretsAuthBrowser:
		if strings.EqualFold(envName, "PRODUCTION") || envName == "" {
			return fmt.Errorf("browser auth flow is not yet available in production; use owner-key-signing")
		}
		return nil
	default:
		return fmt.Errorf("unknown --secrets-auth value %q; expected %q or %q", flow, SecretsAuthOwnerKeySigning, SecretsAuthBrowser)
	}
}

// IsBrowserFlow returns true when the browser (JWT) auth flow is selected.
func IsBrowserFlow(flow string) bool {
	return flow == SecretsAuthBrowser
}

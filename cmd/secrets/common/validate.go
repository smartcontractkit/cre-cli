package common

import (
	"fmt"
	"strings"
)

const (
	SecretsAuthOnchain = "onchain"
	SecretsAuthBrowser = "browser"
)

// ValidateSecretsAuthFlow checks that the chosen auth flow is valid and
// allowed in the current environment. Browser flow is blocked in production.
func ValidateSecretsAuthFlow(flow, envName string) error {
	switch flow {
	case SecretsAuthOnchain:
		return nil
	case SecretsAuthBrowser:
		if strings.EqualFold(envName, "PRODUCTION") || envName == "" {
			return fmt.Errorf("browser auth flow is not yet available in production; use --secrets-auth=onchain")
		}
		return nil
	default:
		return fmt.Errorf("unknown --secrets-auth value %q; expected %q or %q", flow, SecretsAuthOnchain, SecretsAuthBrowser)
	}
}

// IsBrowserFlow returns true when the browser (JWT) auth flow is selected.
func IsBrowserFlow(flow string) bool {
	return flow == SecretsAuthBrowser
}

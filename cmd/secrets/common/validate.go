package common

import "fmt"

const (
	SecretsAuthOnchain = "onchain"
	SecretsAuthBrowser = "browser"
)

// ValidateSecretsAuthFlow checks that the chosen auth flow is valid.
func ValidateSecretsAuthFlow(flow string) error {
	switch flow {
	case SecretsAuthOnchain, SecretsAuthBrowser:
		return nil
	default:
		return fmt.Errorf("unknown --secrets-auth value %q; expected %q or %q", flow, SecretsAuthOnchain, SecretsAuthBrowser)
	}
}

// IsBrowserFlow returns true when the browser (JWT) auth flow is selected.
func IsBrowserFlow(flow string) bool {
	return flow == SecretsAuthBrowser
}

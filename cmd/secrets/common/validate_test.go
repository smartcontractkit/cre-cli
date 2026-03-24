package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateSecretsAuthFlow(t *testing.T) {
	tests := []struct {
		name    string
		flow    string
		env     string
		wantErr bool
		errMsg  string
	}{
		{"owner-key-signing in production", SecretsAuthOwnerKeySigning, "PRODUCTION", false, ""},
		{"owner-key-signing in staging", SecretsAuthOwnerKeySigning, "STAGING", false, ""},
		{"owner-key-signing in dev", SecretsAuthOwnerKeySigning, "DEVELOPMENT", false, ""},
		{"owner-key-signing empty env defaults safe", SecretsAuthOwnerKeySigning, "", false, ""},
		{"browser in staging", SecretsAuthBrowser, "STAGING", false, ""},
		{"browser in dev", SecretsAuthBrowser, "DEVELOPMENT", false, ""},
		{"browser in production blocked", SecretsAuthBrowser, "PRODUCTION", true, "not yet available in production"},
		{"browser in production lowercase", SecretsAuthBrowser, "production", true, "not yet available in production"},
		{"browser empty env treated as production", SecretsAuthBrowser, "", true, "not yet available in production"},
		{"unknown value rejected", "magic", "STAGING", true, "unknown --secrets-auth value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretsAuthFlow(tt.flow, tt.env)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsBrowserFlow(t *testing.T) {
	assert.False(t, IsBrowserFlow(SecretsAuthOwnerKeySigning), "owner-key-signing should not be browser flow")
	assert.True(t, IsBrowserFlow(SecretsAuthBrowser), "browser should be browser flow")
	assert.False(t, IsBrowserFlow("unknown"), "unknown should not be browser flow")
}

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
		wantErr bool
		errMsg  string
	}{
		{"onchain", SecretsAuthOnchain, false, ""},
		{"browser", SecretsAuthBrowser, false, ""},
		{"unknown value rejected", "magic", true, "unknown --secrets-auth value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretsAuthFlow(tt.flow)
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
	assert.False(t, IsBrowserFlow(SecretsAuthOnchain), "onchain should not be browser flow")
	assert.True(t, IsBrowserFlow(SecretsAuthBrowser), "browser should be browser flow")
	assert.False(t, IsBrowserFlow("unknown"), "unknown should not be browser flow")
}

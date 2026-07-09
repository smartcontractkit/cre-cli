package settings_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestResolveEthPrivateKeyFromEnv(t *testing.T) {
	t.Parallel()

	validKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
		errSub  string
	}{
		{
			name: "empty",
			raw:  "",
			want: "",
		},
		{
			name: "cre init placeholder",
			raw:  settings.DefaultEthPrivateKeyEnvPlaceholder,
			want: "",
		},
		{
			name:    "non-hex template text",
			raw:     "invalid",
			wantErr: true,
			errSub:  "invalid private key: expected 64 hex characters",
		},
		{
			name: "valid key without prefix",
			raw:  validKey,
			want: validKey,
		},
		{
			name: "valid key with 0x prefix",
			raw:  "0x" + validKey,
			want: validKey,
		},
		{
			name:    "malformed hex length",
			raw:     strings.Repeat("a", 40),
			wantErr: true,
			errSub:  "invalid private key: expected 64 hex characters",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := settings.ResolveEthPrivateKeyFromEnv(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errSub != "" {
					assert.Contains(t, err.Error(), tc.errSub)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, settings.EthPrivateKeyHex(tc.want), got)
		})
	}
}

package settings

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsMultisigMode(t *testing.T) {
	t.Run("unsigned", func(t *testing.T) {
		v := viper.New()
		v.Set(Flags.RawTxFlag.Name, true)
		assert.True(t, IsMultisigMode(v))
	})

	t.Run("changeset", func(t *testing.T) {
		v := viper.New()
		v.Set(Flags.Changeset.Name, true)
		assert.True(t, IsMultisigMode(v))
	})

	t.Run("neither flag", func(t *testing.T) {
		v := viper.New()
		assert.False(t, IsMultisigMode(v))
	})
}

func secretsCmd(secretsAuth string) *cobra.Command {
	cmd := &cobra.Command{Use: "create"}
	cmd.Flags().String("secrets-auth", secretsAuth, "auth mode")
	return cmd
}

func workflowCmd() *cobra.Command {
	return &cobra.Command{Use: "deploy"}
}

func TestValidateMultisigCompatibility(t *testing.T) {
	offChain := NewOffChainRegistry("42", "test-don")
	onChain := NewOnChainRegistry("mainnet", "0xabc", "ethereum-mainnet", "test-don", "")

	tests := []struct {
		name             string
		unsigned         bool
		changeset        bool
		cmd              *cobra.Command
		resolvedRegistry ResolvedRegistry
		wantErr          bool
		errMsg           string
	}{
		{"not multisig secrets onchain off-chain", false, false, secretsCmd("onchain"), offChain, false, ""},
		{"not multisig secrets browser off-chain", false, false, secretsCmd("browser"), offChain, false, ""},
		{"unsigned secrets onchain on-chain", true, false, secretsCmd("onchain"), onChain, false, ""},
		{"unsigned secrets onchain nil registry", true, false, secretsCmd("onchain"), nil, false, ""},
		{"changeset secrets browser off-chain", false, true, secretsCmd("browser"), offChain, true, "browser secrets auth cannot be combined with multisig"},
		{"unsigned secrets browser on-chain", true, false, secretsCmd("browser"), onChain, true, "browser secrets auth cannot be combined with multisig"},
		{"unsigned secrets onchain off-chain", true, false, secretsCmd("onchain"), offChain, true, "not supported with private registry"},
		{"changeset workflow off-chain", false, true, workflowCmd(), offChain, true, "not supported with private registry"},
		{"unsigned workflow on-chain", true, false, workflowCmd(), onChain, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			if tt.unsigned {
				v.Set(Flags.RawTxFlag.Name, true)
			}
			if tt.changeset {
				v.Set(Flags.Changeset.Name, true)
			}

			err := ValidateMultisigCompatibility(v, tt.cmd, tt.resolvedRegistry)
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

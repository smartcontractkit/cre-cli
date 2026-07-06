package settings_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/rpc"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestValidateDeploymentRPCCleartext(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	config := &settings.WorkflowSettings{
		RPCs: []settings.RpcEndpoint{
			{
				ChainName: "ethereum-testnet-sepolia",
				Url:       "http://rpc.example.com/v3/secret-key",
			},
		},
	}

	t.Run("blocks remote cleartext without opt-in", func(t *testing.T) {
		t.Parallel()
		cleartext := &rpc.CleartextPolicyOptions{}
		err := settings.ValidateDeploymentRPC(config, "ethereum-testnet-sepolia", &logger, cleartext)
		require.Error(t, err)
		require.Contains(t, err.Error(), "--allow-insecure-rpc")
	})

	t.Run("allows remote cleartext with opt-in", func(t *testing.T) {
		t.Parallel()
		cleartext := &rpc.CleartextPolicyOptions{AllowInsecure: true}
		err := settings.ValidateDeploymentRPC(config, "ethereum-testnet-sepolia", &logger, cleartext)
		require.NoError(t, err)
	})
}

func TestValidateSettingsCleartextBlocksRemoteHTTP(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set(settings.Flags.AllowInsecureRPC.Name, false)

	logger := zerolog.Nop()
	config := &settings.WorkflowSettings{
		RPCs: []settings.RpcEndpoint{
			{
				ChainName: "ethereum-testnet-sepolia",
				Url:       "http://rpc.example.com",
			},
		},
	}

	err := settings.ValidateSettingsCleartext(&logger, v, config)
	require.Error(t, err)
	require.Contains(t, err.Error(), "--allow-insecure-rpc")
}

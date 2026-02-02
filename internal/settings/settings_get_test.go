package settings_test

import (
	"fmt"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestGetExperimentalChains(t *testing.T) {
	t.Run("returns experimental chains from rpcs with chain-selector set", func(t *testing.T) {
		v := viper.New()
		v.Set(settings.CreTargetEnvVar, "staging")
		v.Set(fmt.Sprintf("staging.%s", settings.RpcsSettingName), []map[string]interface{}{
			{
				"chain-name":     "ethereum-testnet-sepolia",
				"url":            "https://sepolia.rpc.org",
				"chain-selector": 0, // not experimental
			},
			{
				"chain-name":     "worldchain-sepolia",
				"url":            "https://worldchain-sepolia.rpc.org",
				"chain-selector": uint64(5299555114858065850),
				"forwarder":      "0x76c9cf548b4179F8901cda1f8623568b58215E62",
			},
		})

		chains, err := settings.GetExperimentalChains(v)
		require.NoError(t, err)
		require.Len(t, chains, 1)
		assert.Equal(t, uint64(5299555114858065850), chains[0].Selector)
		assert.Equal(t, "https://worldchain-sepolia.rpc.org", chains[0].RPCURL)
		assert.Equal(t, "0x76c9cf548b4179F8901cda1f8623568b58215E62", chains[0].Forwarder)
	})

	t.Run("returns error when chain-selector is set but forwarder is empty", func(t *testing.T) {
		v := viper.New()
		v.Set(settings.CreTargetEnvVar, "staging")
		v.Set(fmt.Sprintf("staging.%s", settings.RpcsSettingName), []map[string]interface{}{
			{
				"chain-name":     "worldchain-sepolia",
				"url":            "https://worldchain-sepolia.rpc.org",
				"chain-selector": uint64(5299555114858065850),
				"forwarder":      "", // empty forwarder should cause error
			},
		})

		chains, err := settings.GetExperimentalChains(v)
		assert.Error(t, err)
		assert.Nil(t, chains)
		assert.Contains(t, err.Error(), "requires forwarder")
	})

	t.Run("returns empty slice when no experimental chains configured", func(t *testing.T) {
		v := viper.New()
		v.Set(settings.CreTargetEnvVar, "staging")
		v.Set(fmt.Sprintf("staging.%s", settings.RpcsSettingName), []map[string]interface{}{
			{
				"chain-name": "ethereum-testnet-sepolia",
				"url":        "https://sepolia.rpc.org",
			},
		})

		chains, err := settings.GetExperimentalChains(v)
		require.NoError(t, err)
		assert.Empty(t, chains)
	})

	t.Run("returns nil when rpcs key is not set", func(t *testing.T) {
		v := viper.New()
		v.Set(settings.CreTargetEnvVar, "staging")

		chains, err := settings.GetExperimentalChains(v)
		require.NoError(t, err)
		assert.Nil(t, chains)
	})

	t.Run("handles multiple experimental chains", func(t *testing.T) {
		v := viper.New()
		v.Set(settings.CreTargetEnvVar, "staging")
		v.Set(fmt.Sprintf("staging.%s", settings.RpcsSettingName), []map[string]interface{}{
			{
				"chain-name":     "experimental-chain-1",
				"url":            "https://chain1.rpc.org",
				"chain-selector": uint64(1111111111),
				"forwarder":      "0x1111111111111111111111111111111111111111",
			},
			{
				"chain-name":     "experimental-chain-2",
				"url":            "https://chain2.rpc.org",
				"chain-selector": uint64(2222222222),
				"forwarder":      "0x2222222222222222222222222222222222222222",
			},
		})

		chains, err := settings.GetExperimentalChains(v)
		require.NoError(t, err)
		require.Len(t, chains, 2)
		assert.Equal(t, uint64(1111111111), chains[0].Selector)
		assert.Equal(t, uint64(2222222222), chains[1].Selector)
	})
}

func TestGetWorkflowOwner(t *testing.T) {
	validPrivKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	expectedOwner := "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

	t.Run("derives owner from eth private key", func(t *testing.T) {
		v := viper.New()
		v.Set(settings.CreTargetEnvVar, "test")
		v.Set(settings.EthPrivateKeyEnvVar, validPrivKey)

		owner, ownerType, err := settings.GetWorkflowOwner(v)
		assert.NoError(t, err)
		assert.Equal(t, expectedOwner, owner)
		assert.Equal(t, constants.WorkflowOwnerTypeEOA, ownerType)
	})

	t.Run("returns error for invalid eth private key", func(t *testing.T) {
		v := viper.New()
		v.Set(settings.CreTargetEnvVar, "test")
		v.Set(settings.EthPrivateKeyEnvVar, "invalid")

		owner, ownerType, err := settings.GetWorkflowOwner(v)
		assert.Error(t, err)
		assert.Equal(t, "", owner)
		assert.Equal(t, "", ownerType)
	})
}

func TestGetTarget_FlagOverridesEnv(t *testing.T) {
	v := viper.New()
	v.Set(settings.Flags.Target.Name, "flagTarget")
	v.Set(settings.CreTargetEnvVar, "envTarget")

	got, err := settings.GetTarget(v)
	assert.NoError(t, err)
	assert.Equal(t, "flagTarget", got)
}

func TestGetTarget_EnvWhenNoFlag(t *testing.T) {
	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "envOnly")

	got, err := settings.GetTarget(v)
	assert.NoError(t, err)
	assert.Equal(t, "envOnly", got)
}

func TestGetTarget_ErrorWhenNeither(t *testing.T) {
	v := viper.New()

	_, err := settings.GetTarget(v)
	assert.Error(t, err)
}

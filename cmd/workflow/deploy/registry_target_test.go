package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/environments"
)

func TestResolveRegistryTarget(t *testing.T) {
	t.Parallel()

	t.Run("default returns onchain target with chain selector", func(t *testing.T) {
		t.Parallel()
		envSet := &environments.EnvironmentSet{
			EnvName:                   "STAGING",
			WorkflowRegistryChainName: "ethereum-testnet-sepolia",
			WorkflowRegistryAddress:   "0x1234567890123456789012345678901234567890",
		}

		target, err := resolveRegistryTarget(false, envSet)
		require.NoError(t, err)
		assert.Equal(t, registryTargetOnchain, target.targetType)
		assert.False(t, target.isPrivate())
	})

	t.Run("preview flag in STAGING returns private target", func(t *testing.T) {
		t.Parallel()
		envSet := &environments.EnvironmentSet{
			EnvName:                   "STAGING",
			WorkflowRegistryChainName: "ethereum-testnet-sepolia",
			WorkflowRegistryAddress:   "0x1234567890123456789012345678901234567890",
		}

		target, err := resolveRegistryTarget(true, envSet)
		require.NoError(t, err)
		assert.Equal(t, registryTargetPrivate, target.targetType)
		assert.True(t, target.isPrivate())
	})

	t.Run("preview flag blocked in PRODUCTION", func(t *testing.T) {
		t.Parallel()
		envSet := &environments.EnvironmentSet{
			EnvName:                   "PRODUCTION",
			WorkflowRegistryChainName: "ethereum-testnet-sepolia",
			WorkflowRegistryAddress:   "0x1234567890123456789012345678901234567890",
		}

		_, err := resolveRegistryTarget(true, envSet)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--preview-private-registry is only available in the STAGING environment")
	})

	t.Run("preview flag blocked when env is empty (defaults to PRODUCTION)", func(t *testing.T) {
		t.Parallel()
		envSet := &environments.EnvironmentSet{
			EnvName:                   "",
			WorkflowRegistryChainName: "ethereum-testnet-sepolia",
			WorkflowRegistryAddress:   "0x1234567890123456789012345678901234567890",
		}

		_, err := resolveRegistryTarget(true, envSet)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--preview-private-registry is only available in the STAGING environment")
	})

	t.Run("preview flag case insensitive for STAGING", func(t *testing.T) {
		t.Parallel()
		envSet := &environments.EnvironmentSet{
			EnvName:                   "staging",
			WorkflowRegistryChainName: "ethereum-testnet-sepolia",
			WorkflowRegistryAddress:   "0x1234567890123456789012345678901234567890",
		}

		target, err := resolveRegistryTarget(true, envSet)
		require.NoError(t, err)
		assert.True(t, target.isPrivate())
	})
}

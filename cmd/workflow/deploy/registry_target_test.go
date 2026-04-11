package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

type fakeFactory struct {
	txType client.TxType
}

func (f fakeFactory) NewWorkflowRegistryV2Client() (*client.WorkflowRegistryV2Client, error) {
	return nil, nil
}

func (f fakeFactory) GetTxType() client.TxType {
	return f.txType
}

func (f fakeFactory) GetSkipConfirmation() bool {
	return false
}

func TestResolveRegistryTarget(t *testing.T) {
	t.Parallel()

	t.Run("default returns onchain target with chain selector", func(t *testing.T) {
		t.Parallel()
		envSet := &environments.EnvironmentSet{
			EnvName:                   "STAGING",
			WorkflowRegistryChainName: "ethereum-testnet-sepolia",
			WorkflowRegistryAddress:   "0x1234567890123456789012345678901234567890",
		}
		factory := fakeFactory{txType: client.Regular}

		target, err := resolveRegistryTarget(false, envSet, factory)
		require.NoError(t, err)
		assert.Equal(t, registryTargetOnchain, target.targetType)
		assert.NotZero(t, target.onchainChainSelector)
		assert.Equal(t, "0x1234567890123456789012345678901234567890", target.onchainAddress)
		assert.False(t, target.isPrivate())
	})

	t.Run("preview flag in STAGING returns private target", func(t *testing.T) {
		t.Parallel()
		envSet := &environments.EnvironmentSet{
			EnvName:                   "STAGING",
			WorkflowRegistryChainName: "ethereum-testnet-sepolia",
			WorkflowRegistryAddress:   "0x1234567890123456789012345678901234567890",
		}
		factory := fakeFactory{txType: client.Regular}

		target, err := resolveRegistryTarget(true, envSet, factory)
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
		factory := fakeFactory{txType: client.Regular}

		_, err := resolveRegistryTarget(true, envSet, factory)
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
		factory := fakeFactory{txType: client.Regular}

		_, err := resolveRegistryTarget(true, envSet, factory)
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
		factory := fakeFactory{txType: client.Regular}

		target, err := resolveRegistryTarget(true, envSet, factory)
		require.NoError(t, err)
		assert.True(t, target.isPrivate())
	})
}

package deploy

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

type fakeFactory struct{}

func (f fakeFactory) NewWorkflowRegistryV2Client() (*client.WorkflowRegistryV2Client, error) {
	return nil, nil
}

func (f fakeFactory) GetTxType() client.TxType {
	return client.Regular
}

func (f fakeFactory) GetSkipConfirmation() bool {
	return false
}

func TestResolveTargetRegistry(t *testing.T) {
	t.Parallel()

	t.Run("returns onchain target and adapter by default", func(t *testing.T) {
		t.Parallel()
		h := &handler{clientFactory: fakeFactory{}}
		target, adapter, err := resolveTargetRegistry(
			false,
			&environments.EnvironmentSet{
				EnvName:                   "STAGING",
				WorkflowRegistryChainName: "ethereum-testnet-sepolia",
				WorkflowRegistryAddress:   "0x1234567890123456789012345678901234567890",
			},
			h,
		)
		require.NoError(t, err)
		assert.Equal(t, registryTargetOnchain, target.targetType)
		_, ok := adapter.(*onchainRegistryAdapter)
		assert.True(t, ok, "expected onchainRegistryAdapter for onchain target")
	})

	t.Run("returns private target and adapter when preview is enabled", func(t *testing.T) {
		t.Parallel()
		target, adapter, err := resolveTargetRegistry(
			true,
			&environments.EnvironmentSet{
				EnvName:                   "STAGING",
				WorkflowRegistryChainName: "ethereum-testnet-sepolia",
				WorkflowRegistryAddress:   "0x1234567890123456789012345678901234567890",
			},
			&handler{clientFactory: fakeFactory{}},
		)
		require.NoError(t, err)
		assert.Equal(t, registryTargetPrivate, target.targetType)
		_, ok := adapter.(*privateRegistryAdapter)
		assert.True(t, ok, "expected privateRegistryAdapter for private target")
	})
}

// fakeAdapter records which methods were called and in what order.
type fakeAdapter struct {
	preChecksErr error
	upsertErr    error
	calls        []string
}

func (f *fakeAdapter) RunPreDeployChecks() error {
	f.calls = append(f.calls, "RunPreDeployChecks")
	return f.preChecksErr
}

func (f *fakeAdapter) Upsert() error {
	f.calls = append(f.calls, "Upsert")
	return f.upsertErr
}

func TestRunDeploy_HaltedSentinel(t *testing.T) {
	t.Parallel()

	adapter := &fakeAdapter{preChecksErr: errDeployHalted}
	err := runDeploy(adapter, &handler{})
	require.NoError(t, err, "errDeployHalted should map to nil return")
	assert.Equal(t, []string{"RunPreDeployChecks"}, adapter.calls)
}

func TestRunDeploy_PreCheckError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("ownership check failed")
	adapter := &fakeAdapter{preChecksErr: expectedErr}
	err := runDeploy(adapter, &handler{})
	require.ErrorIs(t, err, expectedErr)
	assert.Equal(t, []string{"RunPreDeployChecks"}, adapter.calls)
}

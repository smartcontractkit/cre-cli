package deploy

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/client"
)

func TestNewRegistryAdapter(t *testing.T) {
	t.Parallel()

	t.Run("returns onchain adapter for onchain target", func(t *testing.T) {
		t.Parallel()
		target := registryTarget{targetType: registryTargetOnchain}
		h := &handler{clientFactory: fakeFactory{txType: client.Regular}}
		adapter := newRegistryAdapter(target, h)
		_, ok := adapter.(*onchainRegistryAdapter)
		assert.True(t, ok, "expected onchainRegistryAdapter for onchain target")
	})

	t.Run("returns private adapter for private target", func(t *testing.T) {
		t.Parallel()
		target := registryTarget{targetType: registryTargetPrivate}
		adapter := newRegistryAdapter(target, &handler{})
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

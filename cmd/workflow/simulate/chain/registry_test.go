package chain

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func resetRegistry() {
	mu.Lock()
	defer mu.Unlock()
	families = make(map[string]ChainFamily)
}

// mockChainFamily is a testify/mock implementation of ChainFamily.
type mockChainFamily struct {
	mock.Mock
}

var _ ChainFamily = (*mockChainFamily)(nil)

func (m *mockChainFamily) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockChainFamily) ResolveClients(v *viper.Viper) (map[uint64]ChainClient, map[uint64]string, error) {
	args := m.Called(v)
	clients, _ := args.Get(0).(map[uint64]ChainClient)
	forwarders, _ := args.Get(1).(map[uint64]string)
	return clients, forwarders, args.Error(2)
}

func (m *mockChainFamily) RegisterCapabilities(ctx context.Context, cfg CapabilityConfig) error {
	args := m.Called(ctx, cfg)
	return args.Error(0)
}

func (m *mockChainFamily) ExecuteTrigger(ctx context.Context, selector uint64, registrationID string, triggerData interface{}) error {
	args := m.Called(ctx, selector, registrationID, triggerData)
	return args.Error(0)
}

func (m *mockChainFamily) ParseTriggerChainSelector(triggerID string) (uint64, bool) {
	args := m.Called(triggerID)
	return args.Get(0).(uint64), args.Bool(1)
}

func (m *mockChainFamily) RunHealthCheck(clients map[uint64]ChainClient) error {
	args := m.Called(clients)
	return args.Error(0)
}

func (m *mockChainFamily) SupportedChains() []ChainConfig {
	args := m.Called()
	result, _ := args.Get(0).([]ChainConfig)
	return result
}

func newMockFamily(name string) *mockChainFamily {
	f := new(mockChainFamily)
	f.On("Name").Return(name)
	return f
}

func TestRegisterAndGet(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	mockFamily := newMockFamily("test")
	Register(mockFamily)

	f, err := Get("test")
	require.NoError(t, err)
	assert.Equal(t, "test", f.Name())
	mockFamily.AssertExpectations(t)
}

func TestGetUnknownFamily(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	_, err := Get("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown chain family")
}

func TestRegisterDuplicatePanics(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register(newMockFamily("dup"))
	assert.Panics(t, func() {
		Register(newMockFamily("dup"))
	})
}

func TestAll(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register(newMockFamily("alpha"))
	Register(newMockFamily("beta"))

	all := All()
	assert.Len(t, all, 2)
	assert.Contains(t, all, "alpha")
	assert.Contains(t, all, "beta")
}

func TestNamesReturnsSorted(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register(newMockFamily("zebra"))
	Register(newMockFamily("alpha"))
	Register(newMockFamily("middle"))

	names := Names()
	assert.Equal(t, []string{"alpha", "middle", "zebra"}, names)
}

func TestGetErrorIncludesRegisteredNames(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register(newMockFamily("evm"))
	Register(newMockFamily("aptos"))

	_, err := Get("solana")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aptos")
	assert.Contains(t, err.Error(), "evm")
}

func TestAllReturnsCopy(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	mockFamily := newMockFamily("original")
	Register(mockFamily)

	all := All()
	delete(all, "original")

	// The registry should still have it
	f, err := Get("original")
	require.NoError(t, err)
	assert.Equal(t, "original", f.Name())
	mockFamily.AssertExpectations(t)
}

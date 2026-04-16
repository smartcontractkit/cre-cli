package chain

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-common/pkg/services"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func resetRegistry() {
	mu.Lock()
	defer mu.Unlock()
	registrations = make(map[string]registration)
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

func (m *mockChainFamily) RegisterCapabilities(ctx context.Context, cfg CapabilityConfig) ([]services.Service, error) {
	args := m.Called(ctx, cfg)
	srvcs, _ := args.Get(0).([]services.Service)
	return srvcs, args.Error(1)
}

func (m *mockChainFamily) ExecuteTrigger(ctx context.Context, selector uint64, registrationID string, triggerData interface{}) error {
	args := m.Called(ctx, selector, registrationID, triggerData)
	return args.Error(0)
}

func (m *mockChainFamily) HasSelector(selector uint64) bool {
	args := m.Called(selector)
	return args.Bool(0)
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

func (m *mockChainFamily) ResolveKey(creSettings *settings.Settings, broadcast bool) (interface{}, error) {
	args := m.Called(creSettings, broadcast)
	return args.Get(0), args.Error(1)
}

func (m *mockChainFamily) ResolveTriggerData(ctx context.Context, selector uint64, params TriggerParams) (interface{}, error) {
	args := m.Called(ctx, selector, params)
	return args.Get(0), args.Error(1)
}

func (m *mockChainFamily) CollectCLIInputs(v *viper.Viper) map[string]string {
	args := m.Called(v)
	result, _ := args.Get(0).(map[string]string)
	return result
}

func newMockFamily(name string) *mockChainFamily {
	f := new(mockChainFamily)
	f.On("Name").Return(name)
	return f
}

// registerMock registers a pre-built mock family and immediately builds it so
// tests can exercise Get/All/Names without wiring a real logger.
func registerMock(name string, family ChainFamily) {
	Register(name, func(*zerolog.Logger) ChainFamily { return family }, nil)
	Build(nil)
}

func TestRegisterAndGet(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	mockFamily := newMockFamily("test")
	registerMock("test", mockFamily)

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

	registerMock("dup", newMockFamily("dup"))
	assert.Panics(t, func() {
		registerMock("dup", newMockFamily("dup"))
	})
}

func TestAll(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	registerMock("alpha", newMockFamily("alpha"))
	registerMock("beta", newMockFamily("beta"))

	all := All()
	assert.Len(t, all, 2)
	assert.Contains(t, all, "alpha")
	assert.Contains(t, all, "beta")
}

func TestNamesReturnsSorted(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	registerMock("zebra", newMockFamily("zebra"))
	registerMock("alpha", newMockFamily("alpha"))
	registerMock("middle", newMockFamily("middle"))

	names := Names()
	assert.Equal(t, []string{"alpha", "middle", "zebra"}, names)
}

func TestGetErrorIncludesRegisteredNames(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	registerMock("evm", newMockFamily("evm"))
	registerMock("aptos", newMockFamily("aptos"))

	_, err := Get("solana")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aptos")
	assert.Contains(t, err.Error(), "evm")
}

func TestRegisterAllCLIFlags_StringAndInt(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register("test", func(*zerolog.Logger) ChainFamily { return newMockFamily("test") }, []CLIFlagDef{
		{Name: "test-hash", Description: "a hash", FlagType: CLIFlagString},
		{Name: "test-index", Description: "an index", DefaultValue: "-1", FlagType: CLIFlagInt},
	})

	cmd := &cobra.Command{Use: "test"}
	RegisterAllCLIFlags(cmd)

	f := cmd.Flags().Lookup("test-hash")
	require.NotNil(t, f)
	assert.Equal(t, "", f.DefValue)
	assert.Equal(t, "a hash", f.Usage)

	f = cmd.Flags().Lookup("test-index")
	require.NotNil(t, f)
	assert.Equal(t, "-1", f.DefValue)
	assert.Equal(t, "an index", f.Usage)
}

func TestRegisterAllCLIFlags_NilFlagDefs(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register("test", func(*zerolog.Logger) ChainFamily { return newMockFamily("test") }, nil)

	cmd := &cobra.Command{Use: "test"}
	RegisterAllCLIFlags(cmd) // should not panic
}

func TestCollectAllCLIInputs_MergesAcrossFamilies(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	fam1 := newMockFamily("alpha")
	fam1.On("CollectCLIInputs", mock.Anything).Return(map[string]string{"key-a": "val-a"})
	registerMock("alpha", fam1)

	fam2 := newMockFamily("beta")
	fam2.On("CollectCLIInputs", mock.Anything).Return(map[string]string{"key-b": "val-b"})
	registerMock("beta", fam2)

	v := viper.New()
	result := CollectAllCLIInputs(v)

	assert.Equal(t, "val-a", result["key-a"])
	assert.Equal(t, "val-b", result["key-b"])
}

func TestCollectAllCLIInputs_EmptyWhenNoInputs(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	fam := newMockFamily("empty")
	fam.On("CollectCLIInputs", mock.Anything).Return(map[string]string{})
	registerMock("empty", fam)

	v := viper.New()
	result := CollectAllCLIInputs(v)
	assert.Empty(t, result)
}

func TestAllReturnsCopy(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	mockFamily := newMockFamily("original")
	registerMock("original", mockFamily)

	all := All()
	delete(all, "original")

	// The registry should still have it
	f, err := Get("original")
	require.NoError(t, err)
	assert.Equal(t, "original", f.Name())
	mockFamily.AssertExpectations(t)
}

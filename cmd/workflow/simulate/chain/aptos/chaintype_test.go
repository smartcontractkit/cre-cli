package aptos

import (
	"context"
	"io"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func nopCommonLogger() logger.Logger { return logger.NewWithSync(io.Discard) }

func newRegistry(t *testing.T) *capabilities.Registry {
	t.Helper()
	return capabilities.NewRegistry(logger.Test(t))
}

func TestResolveKey_SentinelUnderBroadcastFails(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	s := &settings.Settings{User: settings.UserSettings{PrivateKeys: map[string]string{settings.Aptos.Name: "0000000000000000000000000000000000000000000000000000000000000001"}}}
	_, err := ct.ResolveKey(s, true)
	require.Error(t, err)
}

func TestResolveKey_UnparseableUnderBroadcastFails(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	s := &settings.Settings{User: settings.UserSettings{PrivateKeys: map[string]string{settings.Aptos.Name: "not-hex"}}}
	_, err := ct.ResolveKey(s, true)
	require.Error(t, err)
}

func TestResolveKey_UnparseableNonBroadcastFallsBackToSentinel(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	s := &settings.Settings{User: settings.UserSettings{PrivateKeys: map[string]string{settings.Aptos.Name: ""}}}
	k, err := ct.ResolveKey(s, false)
	require.NoError(t, err)
	assert.NotNil(t, k)
}

func TestResolveKey_ValidKeyBroadcast(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	s := &settings.Settings{User: settings.UserSettings{PrivateKeys: map[string]string{settings.Aptos.Name: "1111111111111111111111111111111111111111111111111111111111111111"}}}
	k, err := ct.ResolveKey(s, true)
	require.NoError(t, err)
	assert.NotNil(t, k)
}

func TestSupports_False(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	assert.False(t, ct.Supports(1))
}

func TestResolveTriggerData_ReturnsNoTriggerSurface(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	_, err := ct.ResolveTriggerData(context.Background(), 1, chain.TriggerParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no trigger surface")
}

func TestExecuteTrigger_ReturnsNoTriggerSurface(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	err := ct.ExecuteTrigger(context.Background(), 1, "tid", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no trigger surface")
}

func TestRegisterCapabilities_WrongClientType(t *testing.T) {
	t.Parallel()
	lg := zerolog.Nop()
	ct := &AptosChainType{log: &lg}
	cfg := chain.CapabilityConfig{
		Clients:    map[uint64]chain.ChainClient{1: "not-an-aptos-client"},
		Forwarders: map[uint64]string{1: "0x1"},
	}
	_, err := ct.RegisterCapabilities(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client for selector 1 is not aptosfakes.AptosClient")
}

func TestRegisterCapabilities_NoClients_ConstructsEmpty(t *testing.T) {
	t.Parallel()
	lg := zerolog.Nop()
	ct := &AptosChainType{log: &lg}
	cfg := chain.CapabilityConfig{
		Clients:    map[uint64]chain.ChainClient{},
		Forwarders: map[uint64]string{},
		Logger:     nopCommonLogger(),
		Registry:   newRegistry(t),
	}
	srvcs, err := ct.RegisterCapabilities(context.Background(), cfg)
	require.NoError(t, err)
	assert.Empty(t, srvcs)
	assert.False(t, ct.Supports(1))
}

func TestRunHealthCheck_PropagatesInvalidClientType(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	err := ct.RunHealthCheck(chain.ResolvedChains{
		Clients: map[uint64]chain.ChainClient{1: "not-aptos"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client type for Aptos chain type")
}

func TestRegisteredInFactoryRegistry(t *testing.T) {
	t.Parallel()
	lg := zerolog.Nop()
	chain.Build(&lg)
	found := false
	for _, n := range chain.Names() {
		if n == "aptos" {
			found = true
			break
		}
	}
	require.True(t, found, "aptos chain type should be registered at init; got %v", chain.Names())

	ct, err := chain.Get("aptos")
	require.NoError(t, err)
	require.Equal(t, "aptos", ct.Name())
}

func TestCollectCLIInputs_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	assert.Empty(t, ct.CollectCLIInputs(nil))
}

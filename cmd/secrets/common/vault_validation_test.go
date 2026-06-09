package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

func testHandlerWithCapReg(t *testing.T, v *viper.Viper, tenantCtx *tenantctx.EnvironmentContext) *Handler {
	t.Helper()
	h, _, _ := newMockHandler(t)
	h.Viper = v
	h.TenantContext = tenantCtx
	return h
}

func TestEnsureVaultValidationOrConsent_RPCConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  "0xaa36a7",
		}))
	}))
	t.Cleanup(server.Close)

	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "staging")
	v.Set("staging.rpcs", []map[string]string{
		{"chain-name": "ethereum-testnet-sepolia", "url": server.URL},
	})

	tenantCtx := &tenantctx.EnvironmentContext{
		CapabilitiesRegistry: &tenantctx.OnChainContract{
			ChainSelector: 16015286601757825753,
			Address:       "0x7f3191EaF73429177bAB3bAc5c36Ed2D5E39985f",
		},
	}

	h := testHandlerWithCapReg(t, v, tenantCtx)

	skip, err := h.EnsureVaultValidationOrConsent(context.Background())
	require.NoError(t, err)
	require.False(t, skip)
	require.False(t, h.SkipVaultValidation())

	rpcURL, ok := h.CapabilitiesRegistryRPC()
	require.True(t, ok)
	require.Equal(t, server.URL, rpcURL)

	skipCached, err := h.EnsureVaultValidationOrConsent(context.Background())
	require.NoError(t, err)
	require.False(t, skipCached)
}

func TestEnsureVaultValidationOrConsent_SkipConfirmationWithoutRPC(t *testing.T) {
	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "staging")
	v.Set(settings.Flags.SkipConfirmation.Name, true)

	tenantCtx := &tenantctx.EnvironmentContext{
		CapabilitiesRegistry: &tenantctx.OnChainContract{
			ChainSelector: 16015286601757825753,
			Address:       "0x7f3191EaF73429177bAB3bAc5c36Ed2D5E39985f",
		},
	}

	h := testHandlerWithCapReg(t, v, tenantCtx)

	skip, err := h.EnsureVaultValidationOrConsent(context.Background())
	require.NoError(t, err)
	require.True(t, skip)
	require.True(t, h.SkipVaultValidation())
	_, ok := h.CapabilitiesRegistryRPC()
	require.False(t, ok)
}

func TestEnsureVaultValidationOrConsent_NonInteractiveWithoutRPC(t *testing.T) {
	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "staging")
	v.Set(settings.Flags.NonInteractive.Name, true)

	tenantCtx := &tenantctx.EnvironmentContext{
		CapabilitiesRegistry: &tenantctx.OnChainContract{
			ChainSelector: 16015286601757825753,
			Address:       "0x7f3191EaF73429177bAB3bAc5c36Ed2D5E39985f",
		},
	}

	h := testHandlerWithCapReg(t, v, tenantCtx)

	_, err := h.EnsureVaultValidationOrConsent(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing RPC for capabilities registry chain")
}

func TestEnsureVaultValidationOrConsent_MissingCapabilitiesRegistry(t *testing.T) {
	v := viper.New()
	h := testHandlerWithCapReg(t, v, &tenantctx.EnvironmentContext{})

	_, err := h.EnsureVaultValidationOrConsent(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "capabilities registry is not configured")
}

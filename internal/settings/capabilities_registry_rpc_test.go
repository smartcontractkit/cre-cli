package settings_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

const sepoliaChainSelector uint64 = 16015286601757825753

func newEthChainIDServer(t *testing.T, chainIDHex string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "eth_chainId", req.Method)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  chainIDHex,
		}))
	}))
}

func TestResolveCapabilitiesRegistryRPC_MissingTenantContext(t *testing.T) {
	_, _, ok, err := settings.ResolveCapabilitiesRegistryRPC(viper.New(), nil)
	require.Error(t, err)
	require.False(t, ok)
	require.Contains(t, err.Error(), "capabilities registry is not configured")
}

func TestResolveCapabilitiesRegistryRPC_NoRPCConfigured(t *testing.T) {
	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "staging")
	v.Set("staging.rpcs", []map[string]string{
		{"chain-name": "ethereum-mainnet", "url": "https://example.invalid"},
	})

	tenantCtx := &tenantctx.EnvironmentContext{
		CapabilitiesRegistry: &tenantctx.OnChainContract{
			ChainSelector: sepoliaChainSelector,
			Address:       "0x7f3191EaF73429177bAB3bAc5c36Ed2D5E39985f",
		},
	}

	rpcURL, chainName, ok, err := settings.ResolveCapabilitiesRegistryRPC(v, tenantCtx)
	require.NoError(t, err)
	require.False(t, ok)
	require.Empty(t, rpcURL)
	require.Equal(t, "ethereum-testnet-sepolia", chainName)
}

func TestResolveCapabilitiesRegistryRPC_ValidRPC(t *testing.T) {
	server := newEthChainIDServer(t, "0xaa36a7") // Sepolia
	t.Cleanup(server.Close)

	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "staging")
	v.Set("staging.rpcs", []map[string]string{
		{"chain-name": "ethereum-testnet-sepolia", "url": server.URL},
	})

	tenantCtx := &tenantctx.EnvironmentContext{
		CapabilitiesRegistry: &tenantctx.OnChainContract{
			ChainSelector: sepoliaChainSelector,
			Address:       "0x7f3191EaF73429177bAB3bAc5c36Ed2D5E39985f",
		},
	}

	rpcURL, chainName, ok, err := settings.ResolveCapabilitiesRegistryRPC(v, tenantCtx)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, server.URL, rpcURL)
	require.Equal(t, "ethereum-testnet-sepolia", chainName)
}

func TestResolveCapabilitiesRegistryRPC_WrongChainID(t *testing.T) {
	server := newEthChainIDServer(t, "0x1") // mainnet
	t.Cleanup(server.Close)

	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "staging")
	v.Set("staging.rpcs", []map[string]string{
		{"chain-name": "ethereum-testnet-sepolia", "url": server.URL},
	})

	tenantCtx := &tenantctx.EnvironmentContext{
		CapabilitiesRegistry: &tenantctx.OnChainContract{
			ChainSelector: sepoliaChainSelector,
			Address:       "0x7f3191EaF73429177bAB3bAc5c36Ed2D5E39985f",
		},
	}

	_, _, ok, err := settings.ResolveCapabilitiesRegistryRPC(v, tenantCtx)
	require.Error(t, err)
	require.False(t, ok)
	require.Contains(t, err.Error(), "RPC URL points to chain ID")
}

func TestResolveCapabilitiesRegistryRPC_InvalidRPCURL(t *testing.T) {
	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "staging")
	v.Set("staging.rpcs", []map[string]string{
		{"chain-name": "ethereum-testnet-sepolia", "url": "not-a-valid-url"},
	})

	tenantCtx := &tenantctx.EnvironmentContext{
		CapabilitiesRegistry: &tenantctx.OnChainContract{
			ChainSelector: sepoliaChainSelector,
			Address:       "0x7f3191EaF73429177bAB3bAc5c36Ed2D5E39985f",
		},
	}

	_, _, ok, err := settings.ResolveCapabilitiesRegistryRPC(v, tenantCtx)
	require.Error(t, err)
	require.False(t, ok)
	require.Contains(t, err.Error(), "invalid RPC URL")
}

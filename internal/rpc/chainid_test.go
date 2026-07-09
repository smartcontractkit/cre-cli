package rpc_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/rpc"
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

func TestQueryEthChainID(t *testing.T) {
	server := newEthChainIDServer(t, "0xaa36a7")
	t.Cleanup(server.Close)

	chainID, err := rpc.QueryEthChainID(server.URL)
	require.NoError(t, err)
	require.Equal(t, uint64(11155111), chainID)
}

func TestValidateMatchesSelector(t *testing.T) {
	t.Run("matching chain ID", func(t *testing.T) {
		server := newEthChainIDServer(t, "0xaa36a7")
		t.Cleanup(server.Close)

		require.NoError(t, rpc.ValidateMatchesSelector(server.URL, sepoliaChainSelector))
	})

	t.Run("mismatched chain ID", func(t *testing.T) {
		server := newEthChainIDServer(t, "0x1")
		t.Cleanup(server.Close)

		err := rpc.ValidateMatchesSelector(server.URL, sepoliaChainSelector)
		require.Error(t, err)
		require.Contains(t, err.Error(), "RPC URL points to chain ID")
	})
}

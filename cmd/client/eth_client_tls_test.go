package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/client"
)

const testChainID = 1337

// newJSONRPCServer starts a TLS server with a self-signed certificate that answers eth_chainId and eth_blockNumber.
func newJSONRPCServer(t *testing.T, requests *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var result string
		switch req.Method {
		case "eth_chainId":
			result = "0x539"
		case "eth_blockNumber":
			result = "0x10"
		default:
			http.Error(w, "unexpected method "+req.Method, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNewSethClientRejectsUntrustedCertificate(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	srv := newJSONRPCServer(t, &requests)

	sethClient, err := client.NewSethClient("", srv.URL, nil, testChainID)
	if err == nil {
		_, err = sethClient.Client.BlockNumber(context.Background())
	}
	require.Error(t, err)
	require.Contains(t, err.Error(), "certificate")
	require.Zero(t, requests.Load(), "no RPC request may reach a server with an untrusted certificate")
}

// Not parallel: swaps http.DefaultTransport so the test server's certificate is trusted.
func TestNewSethClientTrustedCertificate(t *testing.T) {
	var requests atomic.Int32
	srv := newJSONRPCServer(t, &requests)

	original := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = original })

	sethClient, err := client.NewSethClient("", srv.URL, nil, testChainID)
	require.NoError(t, err)

	block, err := sethClient.Client.BlockNumber(context.Background())
	require.NoError(t, err)
	require.Equal(t, uint64(16), block)
	require.Equal(t, srv.URL, sethClient.URL)
	require.NotNil(t, sethClient.Tracer, "tracer is needed to decode reverted transactions")
}

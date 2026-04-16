package evm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

const selectorSepolia uint64 = 16015286601757825753 // expects "ethereum-testnet-sepolia"

// newChainIDServer returns a JSON-RPC 2.0 server that replies to eth_chainId.
func newChainIDServer(t *testing.T, reply interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")

		type rpcErr struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}

		res := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
		}
		switch v := reply.(type) {
		case string:
			res["result"] = v
		case error:
			res["error"] = rpcErr{Code: -32603, Message: v.Error()}
		default:
			res["result"] = v
		}
		_ = json.NewEncoder(w).Encode(res)
	}))
}

func newEthClient(t *testing.T, url string) *ethclient.Client {
	t.Helper()
	c, err := ethclient.Dial(url)
	if err != nil {
		t.Fatalf("dial eth client: %v", err)
	}
	return c
}

func mustContain(t *testing.T, s string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			t.Fatalf("expected error to contain %q, got:\n%s", sub, s)
		}
	}
}

func TestHealthCheck_NoClientsConfigured(t *testing.T) {
	err := checkRPCConnectivity(map[uint64]*ethclient.Client{}, nil)
	if err == nil {
		t.Fatalf("expected error for no clients configured")
	}
	mustContain(t, err.Error(), "check your settings: no RPC URLs found for supported or experimental chains")
}

func TestHealthCheck_NilClient(t *testing.T) {
	err := checkRPCConnectivity(map[uint64]*ethclient.Client{
		123: nil,
	}, nil)
	if err == nil {
		t.Fatalf("expected error for nil client")
	}
	mustContain(t, err.Error(), "RPC health check failed", "[123] nil client")
}

func TestHealthCheck_AllOK(t *testing.T) {
	sOK := newChainIDServer(t, "0xaa36a7")
	defer sOK.Close()

	cOK := newEthClient(t, sOK.URL)
	defer cOK.Close()

	err := checkRPCConnectivity(map[uint64]*ethclient.Client{
		selectorSepolia: cOK,
	}, nil)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestHealthCheck_RPCError_usesChainName(t *testing.T) {
	sErr := newChainIDServer(t, fmt.Errorf("boom"))
	defer sErr.Close()

	cErr := newEthClient(t, sErr.URL)
	defer cErr.Close()

	err := checkRPCConnectivity(map[uint64]*ethclient.Client{
		selectorSepolia: cErr,
	}, nil)
	if err == nil {
		t.Fatalf("expected error for RPC failure")
	}
	mustContain(t, err.Error(),
		"RPC health check failed",
		"[ethereum-testnet-sepolia] failed RPC health check",
	)
}

func TestHealthCheck_ZeroChainID_usesChainName(t *testing.T) {
	sZero := newChainIDServer(t, "0x0")
	defer sZero.Close()

	cZero := newEthClient(t, sZero.URL)
	defer cZero.Close()

	err := checkRPCConnectivity(map[uint64]*ethclient.Client{
		selectorSepolia: cZero,
	}, nil)
	if err == nil {
		t.Fatalf("expected error for zero chain id")
	}
	mustContain(t, err.Error(),
		"RPC health check failed",
		"[ethereum-testnet-sepolia] invalid RPC response: empty or zero chain ID",
	)
}

func TestHealthCheck_AggregatesMultipleErrors(t *testing.T) {
	sErr := newChainIDServer(t, fmt.Errorf("boom"))
	defer sErr.Close()

	cErr := newEthClient(t, sErr.URL)
	defer cErr.Close()

	err := checkRPCConnectivity(map[uint64]*ethclient.Client{
		selectorSepolia: cErr,
		777:             nil,
	}, nil)
	if err == nil {
		t.Fatalf("expected aggregated error")
	}
	mustContain(t, err.Error(),
		"RPC health check failed",
		"[ethereum-testnet-sepolia] failed RPC health check",
		"[777] nil client",
	)
}

func TestRunRPCHealthCheck_InvalidClientType(t *testing.T) {
	err := RunRPCHealthCheck(map[uint64]chain.ChainClient{
		123: "not-an-ethclient",
	}, nil)
	if err == nil {
		t.Fatalf("expected error for invalid client type")
	}
	mustContain(t, err.Error(), "invalid client type for EVM chain type")
}

package simulate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
)

func TestParseChainSelectorFromTriggerID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want uint64
		ok   bool
	}{
		{
			name: "mainnet format",
			id:   "evm:ChainSelector:5009297550715157269@1.0.0 LogTrigger",
			want: uint64(5009297550715157269),
			ok:   true,
		},
		{
			name: "sepolia lowercase",
			id:   "evm:chainselector:16015286601757825753@1.0.0",
			want: uint64(16015286601757825753),
			ok:   true,
		},
		{
			name: "sepolia uppercase",
			id:   "EVM:CHAINSELECTOR:16015286601757825753@1.0.0",
			want: uint64(16015286601757825753),
			ok:   true,
		},
		{
			name: "leading and trailing spaces",
			id:   "   evm:ChainSelector:123@1.0.0   ",
			want: uint64(123),
			ok:   true,
		},
		{
			name: "no selector present",
			id:   "evm@1.0.0 LogTrigger",
			want: 0,
			ok:   false,
		},
		{
			name: "non-numeric selector",
			id:   "evm:ChainSelector:notanumber@1.0.0",
			want: 0,
			ok:   false,
		},
		{
			name: "empty selector",
			id:   "evm:ChainSelector:@1.0.0",
			want: 0,
			ok:   false,
		},
		{
			name: "overflow uint64",
			// 2^64 is overflow for uint64 (max is 2^64-1)
			id:   "evm:ChainSelector:18446744073709551616@1.0.0",
			want: 0,
			ok:   false,
		},
		{
			name: "digits followed by letters (regex grabs only digits)",
			id:   "evm:ChainSelector:987abc@1.0.0",
			want: uint64(987),
			ok:   true,
		},
		{
			name: "multiple occurrences - returns first",
			id:   "foo ChainSelector:1 bar ChainSelector:2 baz",
			want: uint64(1),
			ok:   true,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseChainSelectorFromTriggerID(tt.id)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("parseChainSelectorFromTriggerID(%q) = (%d, %v); want (%d, %v)", tt.id, got, ok, tt.want, tt.ok)
			}
		})
	}
}

const selectorSepolia uint64 = 16015286601757825753 // expects "ethereum-testnet-sepolia"

// newChainIDServer returns a JSON-RPC 2.0 server that replies to eth_chainId.
// reply can be: string (hex like "0x1" or "0x0") or error (JSON-RPC error).
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
	err := runRPCHealthCheck(map[uint64]*ethclient.Client{})
	if err == nil {
		t.Fatalf("expected error for no clients configured")
	}
	mustContain(t, err.Error(), "check your settings: no RPC URLs found for supported chains")
}

func TestHealthCheck_NilClient(t *testing.T) {
	err := runRPCHealthCheck(map[uint64]*ethclient.Client{
		123: nil, // resolver is not called for nil clients
	})
	if err == nil {
		t.Fatalf("expected error for nil client")
	}
	// nil-client path renders numeric selector in brackets
	mustContain(t, err.Error(), "RPC health check failed", "[123] nil client")
}

func TestHealthCheck_AllOK(t *testing.T) {
	// Any positive chain ID works; use Sepolia id (0xaa36a7 == 11155111) for realism
	sOK := newChainIDServer(t, "0xaa36a7")
	defer sOK.Close()

	cOK := newEthClient(t, sOK.URL)
	defer cOK.Close()

	err := runRPCHealthCheck(map[uint64]*ethclient.Client{
		selectorSepolia: cOK,
	})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestHealthCheck_RPCError_usesChainName(t *testing.T) {
	sErr := newChainIDServer(t, fmt.Errorf("boom"))
	defer sErr.Close()

	cErr := newEthClient(t, sErr.URL)
	defer cErr.Close()

	err := runRPCHealthCheck(map[uint64]*ethclient.Client{
		selectorSepolia: cErr,
	})
	if err == nil {
		t.Fatalf("expected error for RPC failure")
	}
	// We assert the friendly chain name appears (from settings)
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

	err := runRPCHealthCheck(map[uint64]*ethclient.Client{
		selectorSepolia: cZero,
	})
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

	err := runRPCHealthCheck(map[uint64]*ethclient.Client{
		selectorSepolia: cErr, // named failure
		777:             nil,  // nil client (numeric selector path)
	})
	if err == nil {
		t.Fatalf("expected aggregated error")
	}
	mustContain(t, err.Error(),
		"RPC health check failed",
		"[ethereum-testnet-sepolia] failed RPC health check",
		"[777] nil client",
	)
}

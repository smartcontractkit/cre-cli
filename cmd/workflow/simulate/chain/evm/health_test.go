package evm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

const (
	selectorSepolia uint64 = 16015286601757825753 // expects "ethereum-testnet-sepolia"
	chainEthMainnet uint64 = 5009297550715157269  // ethereum-mainnet
)

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
	mustContain(t, err.Error(), "[123] nil client")
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

func TestHealthCheck_ExperimentalSelector_UsesExperimentalLabel(t *testing.T) {
	sErr := newChainIDServer(t, fmt.Errorf("boom"))
	defer sErr.Close()
	c := newEthClient(t, sErr.URL)
	defer c.Close()

	const expSel uint64 = 99999999
	err := checkRPCConnectivity(
		map[uint64]*ethclient.Client{expSel: c},
		map[uint64]bool{expSel: true},
	)
	require.Error(t, err)
	mustContain(t, err.Error(),
		"[experimental chain 99999999]",
	)
}

func TestHealthCheck_ExperimentalSelector_ZeroChainID_UsesExperimentalLabel(t *testing.T) {
	sZero := newChainIDServer(t, "0x0")
	defer sZero.Close()
	c := newEthClient(t, sZero.URL)
	defer c.Close()

	const expSel uint64 = 42424242
	err := checkRPCConnectivity(
		map[uint64]*ethclient.Client{expSel: c},
		map[uint64]bool{expSel: true},
	)
	require.Error(t, err)
	mustContain(t, err.Error(),
		"[experimental chain 42424242]",
		"invalid RPC response: empty or zero chain ID",
	)
}

func TestHealthCheck_UnknownSelector_FallsBackToSelectorLabel(t *testing.T) {
	sErr := newChainIDServer(t, fmt.Errorf("boom"))
	defer sErr.Close()
	c := newEthClient(t, sErr.URL)
	defer c.Close()

	const unknown uint64 = 11111
	err := checkRPCConnectivity(
		map[uint64]*ethclient.Client{unknown: c},
		nil,
	)
	require.Error(t, err)
	mustContain(t, err.Error(),
		fmt.Sprintf("[chain %d]", unknown),
	)
}

func TestHealthCheck_MixedKnownAndExperimental(t *testing.T) {
	sOK := newChainIDServer(t, "0xaa36a7")
	defer sOK.Close()
	cOK := newEthClient(t, sOK.URL)
	defer cOK.Close()

	sErr := newChainIDServer(t, fmt.Errorf("boom"))
	defer sErr.Close()
	cErr := newEthClient(t, sErr.URL)
	defer cErr.Close()

	const expSel uint64 = 99999999
	err := checkRPCConnectivity(
		map[uint64]*ethclient.Client{
			selectorSepolia: cOK,
			expSel:          cErr,
		},
		map[uint64]bool{expSel: true},
	)
	require.Error(t, err)
	mustContain(t, err.Error(),
		"[experimental chain 99999999] failed RPC health check",
	)
	// sepolia is healthy; its label must not appear.
	assert.NotContains(t, err.Error(), "[ethereum-testnet-sepolia] failed")
}

// RunRPCHealthCheck (public wrapper) — ensures ChainClient map conversion.
func TestRunRPCHealthCheck_WrapperConvertsEthClientMap(t *testing.T) {
	sOK := newChainIDServer(t, "0xaa36a7")
	defer sOK.Close()
	c := newEthClient(t, sOK.URL)
	defer c.Close()

	err := RunRPCHealthCheck(
		map[uint64]chain.ChainClient{selectorSepolia: c},
		map[uint64]bool{},
	)
	require.NoError(t, err)
}

func TestHealthCheck_ThreeErrors_AllLabelsInAggregated(t *testing.T) {
	sErr1 := newChainIDServer(t, fmt.Errorf("boom1"))
	defer sErr1.Close()
	cErr1 := newEthClient(t, sErr1.URL)
	defer cErr1.Close()

	sErr2 := newChainIDServer(t, fmt.Errorf("boom2"))
	defer sErr2.Close()
	cErr2 := newEthClient(t, sErr2.URL)
	defer cErr2.Close()

	err := checkRPCConnectivity(
		map[uint64]*ethclient.Client{
			selectorSepolia: cErr1,
			chainEthMainnet: cErr2,
			77777:           nil,
		},
		nil,
	)
	require.Error(t, err)
	mustContain(t, err.Error(),
		"[ethereum-testnet-sepolia] failed RPC health check",
		"[ethereum-mainnet] failed RPC health check",
		"[77777] nil client",
	)
}

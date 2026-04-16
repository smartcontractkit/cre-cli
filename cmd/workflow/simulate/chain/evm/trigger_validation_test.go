package evm

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const zero64 = "0x" + "0000000000000000000000000000000000000000000000000000000000000000"

// ---------------------------------------------------------------------------
// GetEVMTriggerLogFromValues — validation table.
// ---------------------------------------------------------------------------

func TestGetEVMTriggerLogFromValues_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		hash   string
		errSub string
	}{
		{"empty string", "", "transaction hash cannot be empty"},
		{"whitespace only", "   ", "transaction hash cannot be empty"},
		{"no 0x prefix, right length", strings.Repeat("a", 66), "must start with 0x"},
		{"0x prefix, too short", "0x" + strings.Repeat("a", 10), "invalid transaction hash length"},
		{"0x prefix, too long", "0x" + strings.Repeat("a", 100), "invalid transaction hash length"},
		{"valid length but 65 chars", "0x" + strings.Repeat("a", 63), "invalid transaction hash length"},
		{"valid length but 67 chars", "0x" + strings.Repeat("a", 65), "invalid transaction hash length"},
		{"uppercase 0X rejected", "0X" + strings.Repeat("a", 64), "must start with 0x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := GetEVMTriggerLogFromValues(context.Background(), nil, tt.hash, 0)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSub)
		})
	}
}

// ---------------------------------------------------------------------------
// fetchAndConvertLog + GetEVMTriggerLogFromValues via mock eth_getTransactionReceipt.
// ---------------------------------------------------------------------------

type mockRPC struct {
	srv      *httptest.Server
	receipts map[string]*types.Receipt
	errFor   map[string]error
}

func newMockRPC(t *testing.T) *mockRPC {
	t.Helper()
	m := &mockRPC{
		receipts: map[string]*types.Receipt{},
		errFor:   map[string]error{},
	}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}

		switch req.Method {
		case "eth_getTransactionReceipt":
			if len(req.Params) == 0 {
				resp["error"] = map[string]any{"code": -32602, "message": "missing params"}
				break
			}
			var hash string
			_ = json.Unmarshal(req.Params[0], &hash)
			if e, ok := m.errFor[strings.ToLower(hash)]; ok {
				resp["error"] = map[string]any{"code": -32603, "message": e.Error()}
				break
			}
			rec, ok := m.receipts[strings.ToLower(hash)]
			if !ok {
				resp["result"] = nil
				break
			}
			resp["result"] = receiptToJSON(rec)
		case "eth_chainId":
			resp["result"] = "0x1"
		default:
			resp["error"] = map[string]any{"code": -32601, "message": "method not found"}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(m.srv.Close)
	return m
}

func receiptToJSON(r *types.Receipt) map[string]any {
	logs := make([]map[string]any, 0, len(r.Logs))
	for _, l := range r.Logs {
		tpcs := make([]string, 0, len(l.Topics))
		for _, t := range l.Topics {
			tpcs = append(tpcs, t.Hex())
		}
		logs = append(logs, map[string]any{
			"address":          l.Address.Hex(),
			"topics":           tpcs,
			"data":             "0x" + common.Bytes2Hex(l.Data),
			"blockNumber":      fmt.Sprintf("0x%x", l.BlockNumber),
			"transactionHash":  l.TxHash.Hex(),
			"transactionIndex": fmt.Sprintf("0x%x", l.TxIndex),
			"blockHash":        l.BlockHash.Hex(),
			"logIndex":         fmt.Sprintf("0x%x", l.Index),
			"removed":          l.Removed,
		})
	}
	return map[string]any{
		"transactionHash":   r.TxHash.Hex(),
		"transactionIndex":  fmt.Sprintf("0x%x", r.TransactionIndex),
		"blockHash":         r.BlockHash.Hex(),
		"blockNumber":       fmt.Sprintf("0x%x", r.BlockNumber),
		"cumulativeGasUsed": fmt.Sprintf("0x%x", r.CumulativeGasUsed),
		"gasUsed":           fmt.Sprintf("0x%x", r.GasUsed),
		"contractAddress":   nil,
		"logs":              logs,
		"logsBloom":         "0x" + strings.Repeat("00", 256),
		"status":            "0x1",
		"type":              "0x0",
		"effectiveGasPrice": "0x0",
	}
}

func addrFromHex(h string) common.Address { return common.HexToAddress(h) }
func hashFromHex(h string) common.Hash    { return common.HexToHash(h) }

func mkReceipt(txHash common.Hash, logs []*types.Log) *types.Receipt {
	return &types.Receipt{
		TxHash:           txHash,
		TransactionIndex: 0,
		BlockHash:        hashFromHex("0xb1"),
		BlockNumber:      big.NewInt(1),
		Logs:             logs,
		Status:           types.ReceiptStatusSuccessful,
	}
}

func TestGetEVMTriggerLogFromValues_FetchError(t *testing.T) {
	t.Parallel()
	m := newMockRPC(t)
	c := newEthClient(t, m.srv.URL)
	defer c.Close()

	txHash := "0x" + strings.Repeat("a", 64)
	m.errFor[strings.ToLower(txHash)] = fmt.Errorf("receipt not found")

	_, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch transaction receipt")
}

func TestGetEVMTriggerLogFromValues_EventIndexOutOfRange(t *testing.T) {
	t.Parallel()
	m := newMockRPC(t)
	c := newEthClient(t, m.srv.URL)
	defer c.Close()

	txHash := "0x" + strings.Repeat("b", 64)
	rec := mkReceipt(hashFromHex(txHash), []*types.Log{
		{
			Address:     addrFromHex("0xabcd0000000000000000000000000000000000ab"),
			Topics:      []common.Hash{hashFromHex("0xaa")},
			Data:        []byte{0x01, 0x02},
			BlockHash:   hashFromHex("0xbb"),
			TxHash:      hashFromHex(txHash),
			BlockNumber: 1,
			TxIndex:     0,
			Index:       0,
		},
	})
	m.receipts[strings.ToLower(txHash)] = rec

	_, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event index 5 out of range")
	assert.Contains(t, err.Error(), "transaction has 1 log events")
}

func TestGetEVMTriggerLogFromValues_ZeroLogs_OutOfRange(t *testing.T) {
	t.Parallel()
	m := newMockRPC(t)
	c := newEthClient(t, m.srv.URL)
	defer c.Close()

	txHash := "0x" + strings.Repeat("c", 64)
	m.receipts[strings.ToLower(txHash)] = mkReceipt(hashFromHex(txHash), nil)

	_, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event index 0 out of range")
	assert.Contains(t, err.Error(), "transaction has 0 log events")
}

func TestGetEVMTriggerLogFromValues_Success(t *testing.T) {
	t.Parallel()
	m := newMockRPC(t)
	c := newEthClient(t, m.srv.URL)
	defer c.Close()

	txHash := "0x" + strings.Repeat("d", 64)
	log0Addr := addrFromHex("0x1111111111111111111111111111111111111111")
	topicSig := hashFromHex("0x" + strings.Repeat("2", 64))
	extraTopic := hashFromHex("0x" + strings.Repeat("3", 64))
	data := []byte{0xde, 0xad, 0xbe, 0xef}

	rec := mkReceipt(hashFromHex(txHash), []*types.Log{
		{
			Address:     log0Addr,
			Topics:      []common.Hash{topicSig, extraTopic},
			Data:        data,
			BlockHash:   hashFromHex("0xbb"),
			TxHash:      hashFromHex(txHash),
			BlockNumber: 42,
			TxIndex:     7,
			Index:       3,
		},
	})
	m.receipts[strings.ToLower(txHash)] = rec

	stdout := captureStdout(t, func() {})
	_ = stdout
	got, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 0)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, log0Addr.Bytes(), got.Address)
	assert.Equal(t, data, got.Data)
	require.Len(t, got.Topics, 2)
	assert.Equal(t, topicSig.Bytes(), got.Topics[0])
	assert.Equal(t, extraTopic.Bytes(), got.Topics[1])
	assert.Equal(t, topicSig.Bytes(), got.EventSig)
	assert.Equal(t, uint32(7), got.TxIndex)
	assert.Equal(t, uint32(3), got.Index)
	require.NotNil(t, got.BlockNumber)
}

func TestGetEVMTriggerLogFromValues_SuccessNoTopicsLeavesEventSigNil(t *testing.T) {
	t.Parallel()
	m := newMockRPC(t)
	c := newEthClient(t, m.srv.URL)
	defer c.Close()

	txHash := "0x" + strings.Repeat("e", 64)
	rec := mkReceipt(hashFromHex(txHash), []*types.Log{
		{
			Address:     addrFromHex("0x2222222222222222222222222222222222222222"),
			Topics:      nil,
			Data:        []byte{0x01},
			BlockHash:   hashFromHex("0xbb"),
			TxHash:      hashFromHex(txHash),
			BlockNumber: 1,
		},
	})
	m.receipts[strings.ToLower(txHash)] = rec

	got, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 0)
	require.NoError(t, err)
	assert.Empty(t, got.Topics)
	assert.Nil(t, got.EventSig)
}

// ---------------------------------------------------------------------------
// ParseTriggerChainSelector — additional coverage.
// ---------------------------------------------------------------------------

func TestParseTriggerChainSelector_AdditionalEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		want uint64
		ok   bool
	}{
		{"zero selector", "evm:ChainSelector:0@1.0.0", 0, true},
		{"max uint64", "evm:ChainSelector:18446744073709551615@1.0.0", 18446744073709551615, true},
		{"max uint64 plus one", "evm:ChainSelector:18446744073709551616@1.0.0", 0, false},
		{"negative not parsed", "evm:ChainSelector:-1@1.0.0", 0, false},
		{"scientific notation not parsed", "evm:ChainSelector:1e5@1.0.0", 1, true},
		{"hex prefix rejected", "evm:ChainSelector:0x10@1.0.0", 0, true},
		{"ChainSelector without colon", "evm:ChainSelector123@1.0.0", 0, false},
		{"mixed case CHAINselector", "evm:CHAINselector:123@1.0.0", 123, true},
		{"empty string", "", 0, false},
		{"only prefix", "evm:ChainSelector:", 0, false},
		{"unicode digits rejected", "evm:ChainSelector:１２３@1.0.0", 0, false},
		{"tab before number", "evm:ChainSelector:\t42@1.0.0", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseTriggerChainSelector(tt.id)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("ParseTriggerChainSelector(%q) = (%d, %v); want (%d, %v)", tt.id, got, ok, tt.want, tt.ok)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Regex directly — defensive coverage.
// ---------------------------------------------------------------------------

func TestChainSelectorRegex_Matches(t *testing.T) {
	t.Parallel()
	matches := chainSelectorRe.FindAllStringSubmatch("ChainSelector:1 chainselector:2 CHAINSELECTOR:3", -1)
	require.Len(t, matches, 3)
	assert.Equal(t, "1", matches[0][1])
	assert.Equal(t, "2", matches[1][1])
	assert.Equal(t, "3", matches[2][1])
}

func TestChainSelectorRegex_NoMatch(t *testing.T) {
	t.Parallel()
	assert.Nil(t, chainSelectorRe.FindStringSubmatch("just-a-string"))
	assert.Nil(t, chainSelectorRe.FindStringSubmatch(""))
}

// ---------------------------------------------------------------------------
// Non-0x rejection fast — doesn't touch RPC.
// ---------------------------------------------------------------------------

func TestGetEVMTriggerLogFromValues_NoRPCWhenHashInvalid(t *testing.T) {
	t.Parallel()
	// Pass nil client; validation should fire before any RPC attempt.
	_, err := GetEVMTriggerLogFromValues(context.Background(), nil, "not-a-hash", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must start with 0x")
}

// ---------------------------------------------------------------------------
// Zero-address log still decodes.
// ---------------------------------------------------------------------------

func TestGetEVMTriggerLogFromValues_ZeroAddressLog(t *testing.T) {
	t.Parallel()
	m := newMockRPC(t)
	c := newEthClient(t, m.srv.URL)
	defer c.Close()

	txHash := "0x" + strings.Repeat("f", 64)
	rec := mkReceipt(hashFromHex(txHash), []*types.Log{
		{
			Address:     addrFromHex(zero64[:42]),
			Topics:      []common.Hash{hashFromHex("0x00")},
			Data:        []byte{},
			BlockHash:   hashFromHex("0xbb"),
			TxHash:      hashFromHex(txHash),
			BlockNumber: 1,
		},
	})
	m.receipts[strings.ToLower(txHash)] = rec

	got, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 0)
	require.NoError(t, err)
	assert.Len(t, got.Address, 20) // 20-byte address always
}

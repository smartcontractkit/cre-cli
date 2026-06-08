package evm

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	evmpb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
)

const zero64 = "0x" + "0000000000000000000000000000000000000000000000000000000000000000"

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
			_, err := GetEVMTriggerLogFromValues(context.Background(), nil, tt.hash, 0, time.Minute)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSub)
		})
	}
}

type mockRPC struct {
	mu          sync.Mutex
	srv         *httptest.Server
	receipts    map[string]*types.Receipt
	errFor      map[string]error
	logs        []*types.Log
	headNumber  uint64
	getLogsHook func() // optional callback invoked on each eth_getLogs call
}

func newMockRPC(t *testing.T) *mockRPC {
	t.Helper()
	m := &mockRPC{
		receipts: map[string]*types.Receipt{},
		errFor:   map[string]error{},
	}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
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
		case "eth_getBlockByNumber":
			m.mu.Lock()
			head := m.headNumber
			m.mu.Unlock()
			resp["result"] = map[string]any{
				"number":           fmt.Sprintf("0x%x", head),
				"hash":             hashFromHex("0xab").Hex(),
				"parentHash":       hashFromHex("0xaa").Hex(),
				"nonce":            "0x0000000000000000",
				"sha3Uncles":       hashFromHex("0x00").Hex(),
				"logsBloom":        "0x" + strings.Repeat("00", 256),
				"transactionsRoot": hashFromHex("0x00").Hex(),
				"stateRoot":        hashFromHex("0x00").Hex(),
				"receiptsRoot":     hashFromHex("0x00").Hex(),
				"miner":            "0x0000000000000000000000000000000000000000",
				"difficulty":       "0x0",
				"extraData":        "0x",
				"size":             "0x0",
				"gasLimit":         "0x0",
				"gasUsed":          "0x0",
				"timestamp":        "0x0",
				"transactions":     []string{},
				"uncles":           []string{},
				"baseFeePerGas":    "0x0",
			}
		case "eth_getLogs":
			if m.getLogsHook != nil {
				m.getLogsHook()
			}
			from, to := parseGetLogsRange(req.Params)
			m.mu.Lock()
			matching := make([]map[string]any, 0, len(m.logs))
			for _, l := range m.logs {
				if from > 0 && l.BlockNumber < from {
					continue
				}
				if to > 0 && l.BlockNumber > to {
					continue
				}
				matching = append(matching, logToJSON(l))
			}
			m.mu.Unlock()
			resp["result"] = matching
		default:
			resp["error"] = map[string]any{"code": -32601, "message": "method not found"}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(m.srv.Close)
	return m
}

// parseGetLogsRange extracts the FromBlock/ToBlock from an eth_getLogs request
// param object. Returns 0 for missing or "latest"/"earliest" tags so the mock
// matches the relevant logs regardless.
func parseGetLogsRange(params []json.RawMessage) (fromBlock, toBlock uint64) {
	if len(params) == 0 {
		return 0, 0
	}
	var arg struct {
		FromBlock string `json:"fromBlock"`
		ToBlock   string `json:"toBlock"`
	}
	if err := json.Unmarshal(params[0], &arg); err != nil {
		return 0, 0
	}
	fromBlock = parseHexBlock(arg.FromBlock)
	toBlock = parseHexBlock(arg.ToBlock)
	return fromBlock, toBlock
}

func parseHexBlock(s string) uint64 {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "0x") {
		return 0
	}
	v := new(big.Int)
	if _, ok := v.SetString(strings.TrimPrefix(s, "0x"), 16); !ok {
		return 0
	}
	return v.Uint64()
}

func logToJSON(l *types.Log) map[string]any {
	tpcs := make([]string, 0, len(l.Topics))
	for _, t := range l.Topics {
		tpcs = append(tpcs, t.Hex())
	}
	return map[string]any{
		"address":          l.Address.Hex(),
		"topics":           tpcs,
		"data":             "0x" + common.Bytes2Hex(l.Data),
		"blockNumber":      fmt.Sprintf("0x%x", l.BlockNumber),
		"transactionHash":  l.TxHash.Hex(),
		"transactionIndex": fmt.Sprintf("0x%x", l.TxIndex),
		"blockHash":        l.BlockHash.Hex(),
		"logIndex":         fmt.Sprintf("0x%x", l.Index),
		"removed":          l.Removed,
	}
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

	_, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 0, time.Minute)
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

	_, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 5, time.Minute)
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

	_, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 0, time.Minute)
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

	got, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 0, time.Minute)
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

	got, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 0, time.Minute)
	require.NoError(t, err)
	assert.Empty(t, got.Topics)
	assert.Nil(t, got.EventSig)
}

func TestGetEVMTriggerLogFromValues_NoRPCWhenHashInvalid(t *testing.T) {
	t.Parallel()
	// Pass nil client; validation should fire before any RPC attempt.
	_, err := GetEVMTriggerLogFromValues(context.Background(), nil, "not-a-hash", 0, time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must start with 0x")
}

func TestDecodeLogTriggerConfig(t *testing.T) {
	t.Parallel()

	t.Run("nil payload errors", func(t *testing.T) {
		t.Parallel()
		_, err := decodeLogTriggerConfig(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no payload")
	})

	t.Run("wrong message type errors", func(t *testing.T) {
		t.Parallel()
		msg, err := anypb.New(&evmpb.Log{})
		require.NoError(t, err)
		_, err = decodeLogTriggerConfig(msg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "FilterLogTriggerRequest")
	})

	t.Run("round-trip", func(t *testing.T) {
		t.Parallel()
		addr := addrFromHex("0x1111111111111111111111111111111111111111")
		eventSig := hashFromHex("0x" + strings.Repeat("2", 64))
		msg, err := anypb.New(&evmpb.FilterLogTriggerRequest{
			Addresses: [][]byte{addr.Bytes()},
			Topics:    []*evmpb.TopicValues{{Values: [][]byte{eventSig.Bytes()}}},
		})
		require.NoError(t, err)

		cfg, err := decodeLogTriggerConfig(msg)
		require.NoError(t, err)
		require.Len(t, cfg.GetAddresses(), 1)
		assert.Equal(t, addr.Bytes(), cfg.GetAddresses()[0])
		require.Len(t, cfg.GetTopics(), 1)
		assert.Equal(t, eventSig.Bytes(), cfg.GetTopics()[0].GetValues()[0])
	})
}

func TestTopicsToFilter(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, topicsToFilter(nil))
	})

	t.Run("empty slot becomes wildcard", func(t *testing.T) {
		t.Parallel()
		sig := hashFromHex("0x" + strings.Repeat("a", 64))
		got := topicsToFilter([]*evmpb.TopicValues{
			{Values: [][]byte{sig.Bytes()}},
			{Values: nil},
		})
		require.Len(t, got, 2)
		require.Len(t, got[0], 1)
		assert.Equal(t, sig, got[0][0])
		assert.Nil(t, got[1])
	})

	t.Run("multiple values per slot", func(t *testing.T) {
		t.Parallel()
		a := hashFromHex("0x" + strings.Repeat("a", 64))
		b := hashFromHex("0x" + strings.Repeat("b", 64))
		got := topicsToFilter([]*evmpb.TopicValues{{Values: [][]byte{a.Bytes(), b.Bytes()}}})
		require.Len(t, got, 1)
		require.Len(t, got[0], 2)
		assert.Equal(t, a, got[0][0])
		assert.Equal(t, b, got[0][1])
	})
}

func TestWaitForEVMTriggerLog_NoAddressesErrors(t *testing.T) {
	t.Parallel()
	_, err := WaitForEVMTriggerLog(context.Background(), nil, WaitForLogConfig{
		Filter: &evmpb.FilterLogTriggerRequest{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing contract addresses")
}

func TestWaitForEVMTriggerLog_ReturnsFirstMatchingLog(t *testing.T) {
	t.Parallel()

	m := newMockRPC(t)
	c := newEthClient(t, m.srv.URL)
	defer c.Close()

	addr := addrFromHex("0xabcd000000000000000000000000000000000001")
	sig := hashFromHex("0x" + strings.Repeat("c", 64))
	m.mu.Lock()
	m.headNumber = 100
	m.logs = []*types.Log{
		{
			Address:     addr,
			Topics:      []common.Hash{sig},
			Data:        []byte{0xab},
			BlockHash:   hashFromHex("0xbb"),
			TxHash:      hashFromHex("0x" + strings.Repeat("d", 64)),
			BlockNumber: 100, // matches the initial head so the first poll finds it
			TxIndex:     2,
			Index:       0,
		},
	}
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got, err := WaitForEVMTriggerLog(ctx, c, WaitForLogConfig{
		Selector: 16015286601757825753, // ethereum-testnet-sepolia
		Filter: &evmpb.FilterLogTriggerRequest{
			Addresses: [][]byte{addr.Bytes()},
			Topics:    []*evmpb.TopicValues{{Values: [][]byte{sig.Bytes()}}},
		},
		PollInterval: 10 * time.Millisecond,
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, addr.Bytes(), got.Address)
	require.Len(t, got.Topics, 1)
	assert.Equal(t, sig.Bytes(), got.Topics[0])
	assert.Equal(t, sig.Bytes(), got.EventSig)
}

// TestWaitForEVMTriggerLog_DoesNotSkipBlocksBetweenPolls validates that the
// poller correctly scans the inclusive [fromBlock, head] range across
// iterations. The bug we're guarding against: advancing fromBlock to the new
// head without first scanning the blocks between the previous head and the
// new head, which dropped events that landed in that window.
func TestWaitForEVMTriggerLog_DoesNotSkipBlocksBetweenPolls(t *testing.T) {
	t.Parallel()

	m := newMockRPC(t)
	c := newEthClient(t, m.srv.URL)
	defer c.Close()

	addr := addrFromHex("0xabcd000000000000000000000000000000000003")
	sig := hashFromHex("0x" + strings.Repeat("e", 64))

	// Simulate the chain advancing by 5 blocks between two consecutive polls
	// and the matching log living in one of those intermediate blocks.
	m.mu.Lock()
	m.headNumber = 100
	m.mu.Unlock()
	matchingLog := &types.Log{
		Address:     addr,
		Topics:      []common.Hash{sig},
		Data:        []byte{0xaa},
		BlockHash:   hashFromHex("0xbe"),
		TxHash:      hashFromHex("0x" + strings.Repeat("f", 64)),
		BlockNumber: 103, // Falls between initial head (100) and post-jump head (105)
		TxIndex:     0,
		Index:       0,
	}

	getLogsCalls := 0
	m.getLogsHook = func() {
		getLogsCalls++
		// After the first poll completes (with no logs), jump head forward
		// and surface the matching log. The next iteration must scan
		// [101, 105], not [105, 105].
		if getLogsCalls == 1 {
			m.mu.Lock()
			m.headNumber = 105
			m.logs = []*types.Log{matchingLog}
			m.mu.Unlock()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got, err := WaitForEVMTriggerLog(ctx, c, WaitForLogConfig{
		Selector: 16015286601757825753,
		Filter: &evmpb.FilterLogTriggerRequest{
			Addresses: [][]byte{addr.Bytes()},
			Topics:    []*evmpb.TopicValues{{Values: [][]byte{sig.Bytes()}}},
		},
		PollInterval: 10 * time.Millisecond,
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.BlockNumber)
	assert.Equal(t, uint64(103), valuespb.NewIntFromBigInt(got.BlockNumber).Uint64())
}

func TestWaitForEVMTriggerLog_CancelsOnContext(t *testing.T) {
	t.Parallel()

	m := newMockRPC(t)
	c := newEthClient(t, m.srv.URL)
	defer c.Close()

	addr := addrFromHex("0xabcd000000000000000000000000000000000002")
	m.mu.Lock()
	m.headNumber = 50
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// No matching logs are ever set; cancel quickly to confirm the wait loop
	// exits when ctx is done (the Ctrl+C analogue).
	go func() {
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()

	_, err := WaitForEVMTriggerLog(ctx, c, WaitForLogConfig{
		Selector: 16015286601757825753,
		Filter: &evmpb.FilterLogTriggerRequest{
			Addresses: [][]byte{addr.Bytes()},
		},
		PollInterval: 20 * time.Millisecond,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestWaitForEVMTriggerLog_RescansForRPCLag simulates an RPC that publishes
// the block header before the log index, then catches up a couple of polls
// later. With the rescan overlap, the wait loop must re-scan blocks that
// initially returned empty and pick up the late-indexed log instead of
// advancing past them.
func TestWaitForEVMTriggerLog_RescansForRPCLag(t *testing.T) {
	t.Parallel()

	m := newMockRPC(t)
	c := newEthClient(t, m.srv.URL)
	defer c.Close()

	addr := addrFromHex("0xabcd000000000000000000000000000000000004")
	sig := hashFromHex("0x" + strings.Repeat("d", 64))

	// Initial chain state: head at 200, no logs visible to eth_getLogs yet.
	m.mu.Lock()
	m.headNumber = 200
	m.mu.Unlock()

	lateLog := &types.Log{
		Address:     addr,
		Topics:      []common.Hash{sig},
		Data:        []byte{0xab},
		BlockHash:   hashFromHex("0xbe"),
		TxHash:      hashFromHex("0x" + strings.Repeat("a", 64)),
		BlockNumber: 200, // the log exists in the current tip block...
		TxIndex:     0,
		Index:       0,
	}

	hookCalls := 0
	m.getLogsHook = func() {
		hookCalls++
		// Publish the log only after the wait loop has already scanned and
		// found nothing. Without rescan-overlap, the loop would advance past
		// block 200 and never see this log.
		if hookCalls == 2 {
			m.mu.Lock()
			m.logs = []*types.Log{lateLog}
			m.mu.Unlock()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got, err := WaitForEVMTriggerLog(ctx, c, WaitForLogConfig{
		Selector: 16015286601757825753,
		Filter: &evmpb.FilterLogTriggerRequest{
			Addresses: [][]byte{addr.Bytes()},
			Topics:    []*evmpb.TopicValues{{Values: [][]byte{sig.Bytes()}}},
		},
		PollInterval: 10 * time.Millisecond,
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.BlockNumber)
	assert.Equal(t, uint64(200), valuespb.NewIntFromBigInt(got.BlockNumber).Uint64())
}

func TestExtraTopicLines(t *testing.T) {
	t.Parallel()

	sig := hashFromHex("0x" + strings.Repeat("1", 64))
	indexed := hashFromHex("0x" + strings.Repeat("2", 64))

	t.Run("nil or single-slot returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, extraTopicLines(nil))
		assert.Nil(t, extraTopicLines([][]common.Hash{{sig}}))
	})

	t.Run("trailing wildcards trimmed", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, extraTopicLines([][]common.Hash{{sig}, nil, nil}))
	})

	t.Run("intermediate wildcard rendered as any", func(t *testing.T) {
		t.Parallel()
		got := extraTopicLines([][]common.Hash{{sig}, nil, {indexed}})
		require.Len(t, got, 2)
		assert.Equal(t, "(any)", got[0])
		assert.Equal(t, indexed.Hex(), got[1])
	})
}

func TestReplayCommandHint(t *testing.T) {
	t.Parallel()
	assert.Contains(t, replayCommandHint("limitator-workflow"), "cre workflow simulate limitator-workflow --evm-tx-hash")
	assert.Contains(t, replayCommandHint(""), "<workflow>")
}

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

	got, err := GetEVMTriggerLogFromValues(context.Background(), c, txHash, 0, time.Minute)
	require.NoError(t, err)
	assert.Len(t, got.Address, 20) // 20-byte address always
}

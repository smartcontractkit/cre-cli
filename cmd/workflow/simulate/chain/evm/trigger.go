package evm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"google.golang.org/protobuf/types/known/anypb"

	evmpb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"

	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// defaultWaitForLogPollInterval is how often WaitForEVMTriggerLog polls the RPC
// for new matching logs. Kept conservative so a typical wait doesn't hammer
// public RPCs.
const defaultWaitForLogPollInterval = 3 * time.Second

// rescanOverlapBlocks is how many blocks behind the chain tip we re-scan on
// every poll iteration. Some public RPCs (notably load-balanced endpoints like
// publicnode) return a fresh block header from eth_blockNumber a few seconds
// before the same block's logs are visible to eth_getLogs. Without this
// overlap, we'd advance fromBlock past a block whose log hadn't been indexed
// yet and never re-check it.
const rescanOverlapBlocks = 5

// WaitForLogConfig describes a workflow's EVM log trigger subscription so the
// simulator can wait for the next matching on-chain event.
type WaitForLogConfig struct {
	Selector     uint64
	Filter       *evmpb.FilterLogTriggerRequest
	WorkflowName string
	// PollInterval overrides the polling cadence for tests. Zero means default.
	PollInterval time.Duration
	// NowBlock overrides the initial "latest block" lookup for tests. When nil,
	// HeaderByNumber(nil) is used.
	NowBlock *big.Int
}

type EVMLogTriggerListener struct {
	ethClient *ethclient.Client
	addresses []common.Address
	topics    [][]common.Hash
	poll      time.Duration
	fromBlock *big.Int
	seen      map[string]struct{}
	heartbeat int
}

func NewEVMLogTriggerListener(ctx context.Context, ethClient *ethclient.Client, cfg WaitForLogConfig) (*EVMLogTriggerListener, error) {
	if cfg.Filter == nil || len(cfg.Filter.GetAddresses()) == 0 {
		return nil, fmt.Errorf("EVM log trigger config is missing contract addresses; cannot wait for a matching event")
	}

	addresses := make([]common.Address, 0, len(cfg.Filter.GetAddresses()))
	for _, a := range cfg.Filter.GetAddresses() {
		addresses = append(addresses, common.BytesToAddress(a))
	}
	topics := topicsToFilter(cfg.Filter.GetTopics())

	printEVMTriggerWaitHeader(cfg.Selector, addresses, topics, cfg.WorkflowName)

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultWaitForLogPollInterval
	}

	var fromBlock *big.Int
	if cfg.NowBlock != nil {
		fromBlock = new(big.Int).Set(cfg.NowBlock)
	} else {
		head, err := ethClient.HeaderByNumber(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch latest block: %w", err)
		}
		fromBlock = new(big.Int).Set(head.Number)
	}
	ui.Dim(fmt.Sprintf("Listening for logs starting at block %s...", fromBlock.String()))

	return &EVMLogTriggerListener{
		ethClient: ethClient,
		addresses: addresses,
		topics:    topics,
		poll:      poll,
		fromBlock: fromBlock,
		seen:      make(map[string]struct{}),
	}, nil
}

func (l *EVMLogTriggerListener) Next(ctx context.Context) (interface{}, error) {
	ticker := time.NewTicker(l.poll)
	defer ticker.Stop()

	for {
		log, err := l.scanOnce(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			ui.Dim(fmt.Sprintf("RPC error while listening for logs: %v (retrying)", err))
		} else if log != nil {
			ui.Success(fmt.Sprintf("Matching EVM log event found at block %d (tx %s, index %d)",
				log.BlockNumber, log.TxHash.Hex(), log.Index))
			return convertGethLog(log)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (l *EVMLogTriggerListener) scanOnce(ctx context.Context) (*types.Log, error) {
	head, err := l.ethClient.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	if head.Number.Cmp(l.fromBlock) < 0 {
		return nil, nil
	}

	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).Set(l.fromBlock),
		ToBlock:   new(big.Int).Set(head.Number),
		Addresses: l.addresses,
		Topics:    l.topics,
	}
	logs, err := l.ethClient.FilterLogs(ctx, query)
	if err != nil {
		return nil, err
	}

	for i := range logs {
		key := logCursorKey(&logs[i])
		if _, ok := l.seen[key]; ok {
			continue
		}
		l.seen[key] = struct{}{}
		return &logs[i], nil
	}

	l.advanceFromBlock(head.Number)
	l.heartbeat++
	if l.heartbeat%10 == 0 {
		ui.Dim(fmt.Sprintf("Still waiting (scanned through block %s)...", head.Number.String()))
	}
	return nil, nil
}

func (l *EVMLogTriggerListener) advanceFromBlock(head *big.Int) {
	nextFrom := new(big.Int).Add(head, big.NewInt(1))
	rescanFrom := new(big.Int).Sub(head, big.NewInt(int64(rescanOverlapBlocks-1)))
	if rescanFrom.Cmp(nextFrom) < 0 {
		nextFrom = rescanFrom
	}
	if nextFrom.Cmp(l.fromBlock) > 0 {
		l.fromBlock = nextFrom
	}
}

func logCursorKey(log *types.Log) string {
	return fmt.Sprintf("%s:%d:%d:%s", log.BlockHash.Hex(), log.BlockNumber, log.Index, log.TxHash.Hex())
}

// WaitForEVMTriggerLog blocks until a log matching the workflow's EVM log
// trigger config appears on chain, then converts and returns it. Cancel ctx
// (e.g. Ctrl+C) to abort the wait.
//
// Status output is written with plain ui.Print/ui.Dim rather than a bubbletea
// spinner. Bubble Tea puts the terminal in raw mode, which strips the OS
// translation of Ctrl+C into SIGINT — so signal.NotifyContext in the simulator
// would never fire while a spinner was active and the wait could not be
// interrupted.
func WaitForEVMTriggerLog(ctx context.Context, ethClient *ethclient.Client, cfg WaitForLogConfig) (*evmpb.Log, error) {
	listener, err := NewEVMLogTriggerListener(ctx, ethClient, cfg)
	if err != nil {
		return nil, err
	}
	log, err := listener.Next(ctx)
	if err != nil {
		return nil, err
	}
	return log.(*evmpb.Log), nil
}

// topicsToFilter converts the protobuf topic-values structure (4 slots of
// possible values per topic) into the [][]common.Hash form ethclient expects.
func topicsToFilter(in []*evmpb.TopicValues) [][]common.Hash {
	if len(in) == 0 {
		return nil
	}
	out := make([][]common.Hash, 0, len(in))
	for _, slot := range in {
		vals := slot.GetValues()
		if len(vals) == 0 {
			out = append(out, nil) // wildcard for this slot
			continue
		}
		hashes := make([]common.Hash, 0, len(vals))
		for _, v := range vals {
			hashes = append(hashes, common.BytesToHash(v))
		}
		out = append(out, hashes)
	}
	return out
}

// decodeLogTriggerConfig unmarshals the TriggerSubscription's Any payload into
// the EVM FilterLogTriggerRequest message. Returns an error if the payload is
// missing or of the wrong message type.
func decodeLogTriggerConfig(payload *anypb.Any) (*evmpb.FilterLogTriggerRequest, error) {
	if payload == nil {
		return nil, fmt.Errorf("trigger subscription has no payload")
	}
	cfg := &evmpb.FilterLogTriggerRequest{}
	if err := payload.UnmarshalTo(cfg); err != nil {
		return nil, fmt.Errorf("payload is not a FilterLogTriggerRequest: %w", err)
	}
	return cfg, nil
}

// printEVMTriggerWaitHeader prints the structured "what we're waiting for"
// summary used in interactive mode when no replay tx hash is provided.
func printEVMTriggerWaitHeader(selector uint64, addresses []common.Address, topics [][]common.Hash, workflowName string) {
	chainName := chainNameFromSelector(selector)
	ui.Line()
	ui.Print(ui.RenderBold("EVM log trigger selected."))
	ui.Print("")
	ui.Print("Waiting for a matching EVM log event...")
	ui.Print(fmt.Sprintf("Chain: %s", chainName))
	ui.Print(fmt.Sprintf("Contract: %s", formatAddresses(addresses)))
	ui.Print(fmt.Sprintf("Event: %s", formatEventTopic(topics)))
	// Surface any extra indexed-arg constraints (topics[1..3]) so users can
	// tell when their tx isn't matching because of an indexed-arg filter,
	// not just the event signature.
	for i, slot := range extraTopicLines(topics) {
		ui.Print(fmt.Sprintf("Topic[%d]: %s", i+1, slot))
	}
	ui.Print("Press Ctrl+C to stop the simulation.")
	ui.Print("")
	ui.Print("If you already have a transaction hash, restart with:")
	ui.Print(fmt.Sprintf("  %s", ui.RenderCommand(replayCommandHint(workflowName))))
	ui.Line()
}

// extraTopicLines formats topics[1..3] for display, returning one string per
// non-empty constraint slot. Empty/wildcard slots between concrete ones are
// rendered as "(any)" so positional context is preserved.
func extraTopicLines(topics [][]common.Hash) []string {
	if len(topics) <= 1 {
		return nil
	}
	// Trim trailing wildcard slots so we don't print "(any)" lines that
	// carry no information.
	lastConcrete := -1
	for i := 1; i < len(topics); i++ {
		if len(topics[i]) > 0 {
			lastConcrete = i
		}
	}
	if lastConcrete < 1 {
		return nil
	}
	out := make([]string, 0, lastConcrete)
	for _, slot := range topics[1 : lastConcrete+1] {
		if len(slot) == 0 {
			out = append(out, "(any)")
			continue
		}
		parts := make([]string, 0, len(slot))
		for _, h := range slot {
			parts = append(parts, h.Hex())
		}
		out = append(out, strings.Join(parts, ", "))
	}
	return out
}

// printEVMTriggerReplayHeader prints the matching summary for the replay path
// (interactive with --evm-tx-hash). Keeps both flows symmetric so users know
// what's about to happen and how to abort.
func printEVMTriggerReplayHeader(selector uint64, txHash string, eventIndex uint64) {
	chainName := chainNameFromSelector(selector)
	ui.Line()
	ui.Print(ui.RenderBold("EVM log trigger selected."))
	ui.Print("")
	ui.Print("Replaying log event from a known transaction...")
	ui.Print(fmt.Sprintf("Chain: %s", chainName))
	ui.Print(fmt.Sprintf("Transaction: %s", txHash))
	ui.Print(fmt.Sprintf("Event index: %d", eventIndex))
	ui.Print("Press Ctrl+C to stop the simulation.")
	ui.Line()
}

func chainNameFromSelector(selector uint64) string {
	if name, err := settings.GetChainNameByChainSelector(selector); err == nil && name != "" {
		return name
	}
	return fmt.Sprintf("chain-selector %d", selector)
}

func formatAddresses(addrs []common.Address) string {
	if len(addrs) == 0 {
		return "(any)"
	}
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		parts = append(parts, a.Hex())
	}
	return strings.Join(parts, ", ")
}

// formatEventTopic renders topic[0] (the event signature hash) if it's set; a
// workflow's EVM log trigger always pins topic[0] to at least one event sig.
func formatEventTopic(topics [][]common.Hash) string {
	if len(topics) == 0 || len(topics[0]) == 0 {
		return "(any)"
	}
	parts := make([]string, 0, len(topics[0]))
	for _, h := range topics[0] {
		parts = append(parts, h.Hex())
	}
	return strings.Join(parts, ", ")
}

func replayCommandHint(workflowName string) string {
	if strings.TrimSpace(workflowName) == "" {
		workflowName = "<workflow>"
	}
	return fmt.Sprintf("cre workflow simulate %s --evm-tx-hash 0x... --evm-event-index 0", workflowName)
}

// GetEVMTriggerLogFromValues fetches a log given tx hash string and event index.
// Used by the deterministic replay path (both interactive and non-interactive).
// receiptTimeout controls how long to wait for the transaction receipt before giving up.
func GetEVMTriggerLogFromValues(ctx context.Context, ethClient *ethclient.Client, txHashStr string, eventIndex uint64, receiptTimeout time.Duration) (*evmpb.Log, error) {
	txHashStr = strings.TrimSpace(txHashStr)
	if txHashStr == "" {
		return nil, fmt.Errorf("transaction hash cannot be empty")
	}
	if !strings.HasPrefix(txHashStr, "0x") {
		return nil, fmt.Errorf("transaction hash must start with 0x")
	}
	if len(txHashStr) != 66 {
		return nil, fmt.Errorf("invalid transaction hash length: expected 66 characters, got %d", len(txHashStr))
	}

	txHash := common.HexToHash(txHashStr)
	return fetchAndConvertLog(ctx, ethClient, txHash, eventIndex, receiptTimeout)
}

// fetchAndConvertLog fetches a transaction receipt log and converts it to the protobuf format.
// receiptTimeout controls how long to wait for the receipt before the context is cancelled.
func fetchAndConvertLog(ctx context.Context, ethClient *ethclient.Client, txHash common.Hash, eventIndex uint64, receiptTimeout time.Duration) (*evmpb.Log, error) {
	receiptSpinner := ui.NewSpinner()
	receiptSpinner.Start(fmt.Sprintf("Fetching transaction receipt for %s...", txHash.Hex()))

	timeoutCtx, cancel := context.WithTimeout(ctx, receiptTimeout)
	txReceipt, err := waitForTransactionReceipt(timeoutCtx, ethClient, txHash)
	receiptSpinner.Stop()
	cancel()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction receipt: %w", err)
	}

	if eventIndex >= uint64(len(txReceipt.Logs)) {
		return nil, fmt.Errorf("event index %d out of range, transaction has %d log events", eventIndex, len(txReceipt.Logs))
	}

	return convertGethLog(txReceipt.Logs[eventIndex])
}

// convertGethLog converts a go-ethereum types.Log into the protobuf evm.Log
// the simulator's trigger pipeline expects.
func convertGethLog(log *types.Log) (*evmpb.Log, error) {
	if log.TxIndex > math.MaxUint32 {
		return nil, fmt.Errorf("transaction index %d exceeds uint32 maximum value", log.TxIndex)
	}
	if log.Index > math.MaxUint32 {
		return nil, fmt.Errorf("log index %d exceeds uint32 maximum value", log.Index)
	}

	pbLog := &evmpb.Log{
		Address:     log.Address.Bytes(),
		Data:        log.Data,
		BlockHash:   log.BlockHash.Bytes(),
		TxHash:      log.TxHash.Bytes(),
		TxIndex:     uint32(log.TxIndex), // #nosec G115 -- validated above
		Index:       uint32(log.Index),   // #nosec G115 -- validated above
		Removed:     log.Removed,
		BlockNumber: valuespb.NewBigIntFromInt(new(big.Int).SetUint64(log.BlockNumber)),
	}
	for _, topic := range log.Topics {
		pbLog.Topics = append(pbLog.Topics, topic.Bytes())
	}
	if len(log.Topics) > 0 {
		pbLog.EventSig = log.Topics[0].Bytes()
	}
	return pbLog, nil
}

func waitForTransactionReceipt(ctx context.Context, ethClient *ethclient.Client, txHash common.Hash) (*types.Receipt, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		txReceipt, err := ethClient.TransactionReceipt(ctx, txHash)
		if err == nil {
			return txReceipt, nil
		}
		if !errors.Is(err, ethereum.NotFound) {
			return nil, err
		}
		// tx not found yet, wait and retry
		time.Sleep(3 * time.Second)
	}
}

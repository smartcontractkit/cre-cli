package evm

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	evmpb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"

	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// getEVMTriggerLog prompts user for EVM trigger data and fetches the log.
func getEVMTriggerLog(ctx context.Context, ethClient *ethclient.Client) (*evmpb.Log, error) {
	var txHashInput string
	var eventIndexInput string

	ui.Line()
	if err := ui.InputForm([]ui.InputField{
		{
			Title:       "EVM Trigger Configuration",
			Description: "Transaction hash for the EVM log event",
			Placeholder: "0x...",
			Value:       &txHashInput,
			Validate: func(s string) error {
				s = strings.TrimSpace(s)
				if s == "" {
					return fmt.Errorf("transaction hash cannot be empty")
				}
				if !strings.HasPrefix(s, "0x") {
					return fmt.Errorf("transaction hash must start with 0x")
				}
				if len(s) != 66 {
					return fmt.Errorf("invalid transaction hash length: expected 66 characters, got %d", len(s))
				}
				return nil
			},
		},
		{
			Title:       "Event Index",
			Description: "Log event index (0-based)",
			Placeholder: "0",
			Suggestions: []string{"0"},
			Value:       &eventIndexInput,
			Validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("event index cannot be empty")
				}
				if _, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32); err != nil {
					return fmt.Errorf("invalid event index: must be a number")
				}
				return nil
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("evm trigger input cancelled: %w", err)
	}

	txHashInput = strings.TrimSpace(txHashInput)
	txHash := common.HexToHash(txHashInput)

	eventIndexInput = strings.TrimSpace(eventIndexInput)
	eventIndex, err := strconv.ParseUint(eventIndexInput, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid event index: %w", err)
	}

	return fetchLogFromReceipt(ctx, ethClient, txHash, eventIndex)
}

// getEVMTriggerLogFromValues fetches a log given tx hash and event index (non-interactive).
func getEVMTriggerLogFromValues(ctx context.Context, ethClient *ethclient.Client, txHashStr string, eventIndex uint64) (*evmpb.Log, error) {
	txHashStr = strings.TrimSpace(txHashStr)
	if txHashStr == "" {
		return nil, fmt.Errorf("transaction hash cannot be empty")
	}
	if !strings.HasPrefix(txHashStr, "0x") {
		return nil, fmt.Errorf("transaction hash must start with 0x")
	}
	if len(txHashStr) != 66 { // 0x + 64 hex chars
		return nil, fmt.Errorf("invalid transaction hash length: expected 66 characters, got %d", len(txHashStr))
	}

	txHash := common.HexToHash(txHashStr)
	return fetchLogFromReceipt(ctx, ethClient, txHash, eventIndex)
}

// fetchLogFromReceipt fetches a transaction receipt and converts the log at the given index to protobuf.
func fetchLogFromReceipt(ctx context.Context, ethClient *ethclient.Client, txHash common.Hash, eventIndex uint64) (*evmpb.Log, error) {
	receiptSpinner := ui.NewSpinner()
	receiptSpinner.Start(fmt.Sprintf("Fetching transaction receipt for %s...", txHash.Hex()))
	txReceipt, err := ethClient.TransactionReceipt(ctx, txHash)
	receiptSpinner.Stop()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction receipt: %w", err)
	}

	if eventIndex >= uint64(len(txReceipt.Logs)) {
		return nil, fmt.Errorf("event index %d out of range, transaction has %d log events", eventIndex, len(txReceipt.Logs))
	}

	log := txReceipt.Logs[eventIndex]
	ui.Success(fmt.Sprintf("Found log event at index %d: contract=%s, topics=%d", eventIndex, log.Address.Hex(), len(log.Topics)))

	// Check for potential uint32 overflow
	var txIndex, logIndex uint32
	if log.TxIndex > math.MaxUint32 {
		return nil, fmt.Errorf("transaction index %d exceeds uint32 maximum value", log.TxIndex)
	}
	txIndex = uint32(log.TxIndex)

	if log.Index > math.MaxUint32 {
		return nil, fmt.Errorf("log index %d exceeds uint32 maximum value", log.Index)
	}
	logIndex = uint32(log.Index)

	pbLog := &evmpb.Log{
		Address:     log.Address.Bytes(),
		Data:        log.Data,
		BlockHash:   log.BlockHash.Bytes(),
		TxHash:      log.TxHash.Bytes(),
		TxIndex:     txIndex,
		Index:       logIndex,
		Removed:     log.Removed,
		BlockNumber: valuespb.NewBigIntFromInt(new(big.Int).SetUint64(log.BlockNumber)),
	}

	for _, topic := range log.Topics {
		pbLog.Topics = append(pbLog.Topics, topic.Bytes())
	}

	if len(log.Topics) > 0 {
		pbLog.EventSig = log.Topics[0].Bytes()
	}

	ui.Success(fmt.Sprintf("Created EVM trigger log for transaction %s, event %d", txHash.Hex(), eventIndex))
	return pbLog, nil
}

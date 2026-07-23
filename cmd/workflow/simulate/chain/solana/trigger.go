package solana

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"strings"

	"github.com/gagliardetto/solana-go"
	solanarpc "github.com/gagliardetto/solana-go/rpc"
	"google.golang.org/protobuf/types/known/anypb"

	solcap "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-solana/pkg/solana/logpoller"
	logpollertypes "github.com/smartcontractkit/chainlink-solana/pkg/solana/logpoller/types"

	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

type anchorEvent struct {
	programID solana.PublicKey
	data      []byte
}

func GetSolanaTriggerLogWithFilter(ctx context.Context, client *solanarpc.Client, sigStr string, eventIndex uint64, filter *solcap.FilterLogTriggerRequest) (*solcap.Log, error) {
	return getSolanaTriggerLogFromValues(ctx, client, sigStr, eventIndex, filter)
}

func getSolanaTriggerLogFromValues(ctx context.Context, client *solanarpc.Client, sigStr string, eventIndex uint64, filter *solcap.FilterLogTriggerRequest) (*solcap.Log, error) {
	sigStr = strings.TrimSpace(sigStr)
	if sigStr == "" {
		return nil, fmt.Errorf("transaction signature cannot be empty")
	}
	sig, err := solana.SignatureFromBase58(sigStr)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction signature %q: %w", sigStr, err)
	}

	maxVer := uint64(0)
	res, err := client.GetTransaction(ctx, sig, &solanarpc.GetTransactionOpts{
		Encoding:                       solana.EncodingBase64,
		Commitment:                     solanarpc.CommitmentConfirmed,
		MaxSupportedTransactionVersion: &maxVer,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction %s: %w", sigStr, err)
	}
	if res == nil || res.Meta == nil {
		return nil, fmt.Errorf("transaction %s has no metadata (not confirmed?)", sigStr)
	}
	if res.Meta.Err != nil {
		return nil, fmt.Errorf("transaction %s failed on-chain and emitted no usable events: %v", sigStr, res.Meta.Err)
	}

	if filter.GetCpiFilterConfig() != nil {
		if res.Transaction == nil {
			return nil, fmt.Errorf("transaction %s has no transaction body", sigStr)
		}
		tx, err := res.Transaction.GetTransaction()
		if err != nil {
			return nil, fmt.Errorf("failed to decode transaction %s: %w", sigStr, err)
		}
		events, err := extractSolanaCPIEvents(tx, res.Meta, filter)
		if err != nil {
			return nil, err
		}
		if len(events) == 0 {
			return nil, fmt.Errorf("transaction %s emitted no matching CPI events", sigStr)
		}
		if eventIndex >= uint64(len(events)) {
			return nil, fmt.Errorf("event index %d out of range, transaction emitted %d matching CPI event(s)", eventIndex, len(events))
		}
		return solanaEventToLog(events[eventIndex], sig, res, eventIndex)
	}

	events, err := parseAnchorEvents(res.Meta.LogMessages)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("transaction %s emitted no Anchor events ('Program data:' log lines)", sigStr)
	}
	if eventIndex >= uint64(len(events)) {
		return nil, fmt.Errorf("event index %d out of range, transaction emitted %d event(s)", eventIndex, len(events))
	}

	return solanaEventToLog(events[eventIndex], sig, res, eventIndex)
}

func solanaEventToLog(ev anchorEvent, sig solana.Signature, res *solanarpc.GetTransactionResult, eventIndex uint64) (*solcap.Log, error) {
	if eventIndex > math.MaxInt64 {
		return nil, fmt.Errorf("event index %d exceeds int64 maximum", eventIndex)
	}

	log := &solcap.Log{
		Address:     ev.programID.Bytes(),
		Data:        ev.data,
		TxHash:      sig[:],
		BlockNumber: int64(res.Slot), // #nosec G115 -- slot fits int64 in practice
		LogIndex:    int64(eventIndex),
	}
	if len(ev.data) >= 8 {
		log.EventSig = ev.data[:8]
	}
	if res.BlockTime != nil {
		log.BlockTimestamp = uint64(res.BlockTime.Time().Unix()) // #nosec G115 -- unix seconds
	}
	return log, nil
}

func extractSolanaCPIEvents(tx *solana.Transaction, meta *solanarpc.TransactionMeta, filter *solcap.FilterLogTriggerRequest) ([]anchorEvent, error) {
	cfg := filter.GetCpiFilterConfig()
	if cfg == nil {
		return nil, nil
	}
	if len(filter.GetAddress()) != solana.PublicKeyLength {
		return nil, fmt.Errorf("CPI filter source address must be %d bytes, got %d", solana.PublicKeyLength, len(filter.GetAddress()))
	}
	if len(cfg.GetDestAddress()) != solana.PublicKeyLength {
		return nil, fmt.Errorf("CPI filter destination address must be %d bytes, got %d", solana.PublicKeyLength, len(cfg.GetDestAddress()))
	}
	if len(cfg.GetMethodName()) == 0 {
		return nil, fmt.Errorf("CPI filter method name cannot be empty")
	}
	if tx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}
	if meta == nil {
		return nil, fmt.Errorf("transaction metadata is nil")
	}

	if string(cfg.GetMethodName()) != logpollertypes.AnchorCPIMethodName {
		return nil, fmt.Errorf("unsupported CPI method name %q, only %q is supported", cfg.GetMethodName(), logpollertypes.AnchorCPIMethodName)
	}

	cpiFilter := logpollertypes.Filter{
		Address: logpollertypes.PublicKey(solana.PublicKeyFromBytes(filter.GetAddress())),
		ExtraFilterConfig: logpollertypes.ExtraFilterConfig{
			DestProgram:     logpollertypes.PublicKey(solana.PublicKeyFromBytes(cfg.GetDestAddress())),
			MethodSignature: logpollertypes.AnchorCPIEventDiscriminator(),
		},
	}

	extractor := logpoller.NewCPIEventExtractor(logger.Sugared(logger.Nop()))
	extractor.AddFilter(cpiFilter)

	programEvents := extractor.ExtractCPIEvents(tx, meta, logpollertypes.BlockData{})
	events := make([]anchorEvent, 0, len(programEvents))
	for _, pe := range programEvents {
		programID, err := solana.PublicKeyFromBase58(pe.Program)
		if err != nil {
			return nil, fmt.Errorf("invalid CPI event source program %q: %w", pe.Program, err)
		}
		data, err := base64.StdEncoding.DecodeString(pe.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to base64-decode CPI event data: %w", err)
		}
		events = append(events, anchorEvent{programID: programID, data: data})
	}
	return events, nil
}

func decodeLogTriggerConfig(payload *anypb.Any) (*solcap.FilterLogTriggerRequest, error) {
	if payload == nil {
		return nil, fmt.Errorf("trigger subscription has no payload")
	}
	cfg := &solcap.FilterLogTriggerRequest{}
	if err := payload.UnmarshalTo(cfg); err != nil {
		return nil, fmt.Errorf("payload is not a FilterLogTriggerRequest: %w", err)
	}
	return cfg, nil
}

// parseAnchorEvents decodes the transaction's "Program data:" log lines via
// chainlink-solana's own log poller parser then flattens its per-instruction grouping
// into a single emission-ordered event list.
func parseAnchorEvents(logs []string) ([]anchorEvent, error) {
	outputs, err := logpoller.ParseProgramLogs(logs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse program logs: %w", err)
	}

	var events []anchorEvent
	for _, out := range outputs {
		for _, ev := range out.Events {
			pid, err := solana.PublicKeyFromBase58(ev.Program)
			if err != nil {
				return nil, fmt.Errorf("invalid emitting program address %q: %w", ev.Program, err)
			}
			data, err := base64.StdEncoding.DecodeString(ev.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to base64-decode event data: %w", err)
			}
			events = append(events, anchorEvent{programID: pid, data: data})
		}
	}
	return events, nil
}

func printSolanaTriggerReplayHeader(selector uint64, sig string, eventIndex uint64) {
	ui.Line()
	ui.Print(ui.RenderBold("Solana log trigger selected."))
	ui.Print("")
	ui.Print("Replaying event from a known transaction...")
	ui.Print(fmt.Sprintf("Chain: %s", chainNameFromSelector(selector)))
	ui.Print(fmt.Sprintf("Transaction: %s", sig))
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

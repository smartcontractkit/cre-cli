package solana

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"strings"

	"github.com/gagliardetto/solana-go"
	solanarpc "github.com/gagliardetto/solana-go/rpc"
	"google.golang.org/protobuf/types/known/anypb"

	solcap "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana"

	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// anchorEventLogPrefix is the log line prefix Anchor uses for emitted events
// (sol_log_data), carrying the base64-encoded event payload.
const anchorEventLogPrefix = "Program data: "

const (
	anchorCPIMethodName       = "anchor:event"
	cpiMethodDiscriminatorLen = 8
)

// anchorEvent is a single decoded "Program data:" event, attributed to the
// program that was executing when it was emitted.
type anchorEvent struct {
	programID solana.PublicKey
	data      []byte
}

// GetSolanaTriggerLogFromValues fetches a confirmed transaction by signature and
// builds a solcap.Log from the eventIndex-th Anchor event ("Program data:" log
// line). Used by the deterministic replay path (both interactive and CI).
func GetSolanaTriggerLogFromValues(ctx context.Context, client *solanarpc.Client, sigStr string, eventIndex uint64) (*solcap.Log, error) {
	return getSolanaTriggerLogFromValues(ctx, client, sigStr, eventIndex, nil)
}

// GetSolanaTriggerLogFromValuesWithFilter fetches a confirmed transaction by
// signature and builds a solcap.Log using the registered trigger filter. CPI
// filters are replayed from transaction inner instructions; all other filters
// use the regular Anchor "Program data:" log path.
func GetSolanaTriggerLogFromValuesWithFilter(ctx context.Context, client *solanarpc.Client, sigStr string, eventIndex uint64, filter *solcap.FilterLogTriggerRequest) (*solcap.Log, error) {
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
	// Anchor prefixes event data with an 8-byte discriminator that identifies the
	// event type; surface it as EventSig (kept in Data as well so the workflow can
	// decode the full payload).
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

	sourceProgram := solana.PublicKeyFromBytes(filter.GetAddress())
	destProgram := solana.PublicKeyFromBytes(cfg.GetDestAddress())
	methodSig := cpiMethodDiscriminator(string(cfg.GetMethodName()))
	allAccountKeys := allSolanaAccountKeys(tx, meta)
	if len(allAccountKeys) == 0 {
		return nil, nil
	}

	var events []anchorEvent
	for _, inner := range meta.InnerInstructions {
		if int(inner.Index) >= len(tx.Message.Instructions) {
			continue
		}
		outerInstruction := tx.Message.Instructions[inner.Index]
		if int(outerInstruction.ProgramIDIndex) >= len(allAccountKeys) {
			continue
		}

		outerProgram := allAccountKeys[outerInstruction.ProgramIDIndex]
		programAtStackHeight := map[uint16]solana.PublicKey{
			1: outerProgram,
		}

		for _, ix := range inner.Instructions {
			if int(ix.ProgramIDIndex) >= len(allAccountKeys) {
				continue
			}
			currentDestProgram := allAccountKeys[ix.ProgramIDIndex]
			if ix.StackHeight > 0 {
				programAtStackHeight[ix.StackHeight] = currentDestProgram
			}
			if currentDestProgram != destProgram {
				continue
			}

			data := []byte(ix.Data)
			if len(data) <= cpiMethodDiscriminatorLen || !bytes.Equal(data[:cpiMethodDiscriminatorLen], methodSig[:]) {
				continue
			}

			currentSourceProgram, ok := cpiSourceProgram(ix.StackHeight, programAtStackHeight, outerProgram)
			if !ok || currentSourceProgram != sourceProgram {
				continue
			}

			eventData, ok := cpiEventData(data, string(cfg.GetMethodName()))
			if !ok || len(eventData) == 0 {
				continue
			}
			events = append(events, anchorEvent{programID: currentSourceProgram, data: eventData})
		}
	}

	return events, nil
}

func allSolanaAccountKeys(tx *solana.Transaction, meta *solanarpc.TransactionMeta) []solana.PublicKey {
	if tx == nil {
		return nil
	}
	capacity := len(tx.Message.AccountKeys)
	if meta != nil {
		capacity += len(meta.LoadedAddresses.Writable) + len(meta.LoadedAddresses.ReadOnly)
	}
	allKeys := make([]solana.PublicKey, 0, capacity)
	allKeys = append(allKeys, tx.Message.AccountKeys...)
	if meta != nil {
		allKeys = append(allKeys, meta.LoadedAddresses.Writable...)
		allKeys = append(allKeys, meta.LoadedAddresses.ReadOnly...)
	}
	return allKeys
}

func cpiSourceProgram(stackHeight uint16, programAtStackHeight map[uint16]solana.PublicKey, outerProgram solana.PublicKey) (solana.PublicKey, bool) {
	switch {
	case stackHeight > 1:
		source, ok := programAtStackHeight[stackHeight-1]
		return source, ok
	case stackHeight == 1:
		return solana.PublicKey{}, false
	default:
		return outerProgram, true
	}
}

// cpiEventData strips the anchor:event CPI method discriminator, returning the
// remaining bytes as the event payload. cre-sdk-go's AnchorCPILogTriggerConfig
// always uses this method name, so no other CPI dispatch convention is supported.
func cpiEventData(data []byte, methodName string) ([]byte, bool) {
	if methodName != anchorCPIMethodName || len(data) <= cpiMethodDiscriminatorLen {
		return nil, false
	}
	return data[cpiMethodDiscriminatorLen:], true
}

// cpiMethodDiscriminator computes the anchor:event self-CPI discriminator
// (sha256("anchor:event")[:8], byte-reversed). Only anchorCPIMethodName is
// supported, matching cre-sdk-go's AnchorCPILogTriggerConfig.
func cpiMethodDiscriminator(methodName string) [cpiMethodDiscriminatorLen]byte {
	sum := sha256.Sum256([]byte(anchorCPIMethodName))
	var sig [cpiMethodDiscriminatorLen]byte
	copy(sig[:], sum[:cpiMethodDiscriminatorLen])
	for i, j := 0, len(sig)-1; i < j; i, j = i+1, j-1 {
		sig[i], sig[j] = sig[j], sig[i]
	}
	return sig
}

// decodeLogTriggerConfig unmarshals the TriggerSubscription's Any payload into
// the Solana FilterLogTriggerRequest message. Returns an error if the payload is
// missing or of the wrong message type.
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

// parseAnchorEvents walks the transaction log messages, tracking the program
// invocation stack so each "Program data:" event is attributed to the program
// that emitted it. Returns events in emission order.
func parseAnchorEvents(logs []string) ([]anchorEvent, error) {
	var stack []solana.PublicKey
	var events []anchorEvent

	for _, line := range logs {
		switch {
		case strings.HasPrefix(line, "Program ") && strings.Contains(line, " invoke ["):
			// "Program <pubkey> invoke [n]"
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if pk, err := solana.PublicKeyFromBase58(fields[1]); err == nil {
					stack = append(stack, pk)
				}
			}
		case strings.HasPrefix(line, "Program ") && (strings.HasSuffix(line, " success") || strings.Contains(line, " failed")):
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case strings.HasPrefix(line, anchorEventLogPrefix):
			b64 := strings.TrimPrefix(line, anchorEventLogPrefix)
			data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
			if err != nil {
				return nil, fmt.Errorf("failed to base64-decode event data: %w", err)
			}
			var pid solana.PublicKey
			if len(stack) > 0 {
				pid = stack[len(stack)-1]
			}
			events = append(events, anchorEvent{programID: pid, data: data})
		}
	}
	return events, nil
}

// printSolanaTriggerReplayHeader prints the "what's about to happen" summary for
// the replay path, mirroring the EVM equivalent.
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

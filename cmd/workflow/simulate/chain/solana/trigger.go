package solana

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"strings"

	"github.com/gagliardetto/solana-go"
	solanarpc "github.com/gagliardetto/solana-go/rpc"

	solcap "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana"

	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// anchorEventLogPrefix is the log line prefix Anchor uses for emitted events
// (sol_log_data), carrying the base64-encoded event payload.
const anchorEventLogPrefix = "Program data: "

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

	ev := events[eventIndex]
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

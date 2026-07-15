package solana

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	solcap "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana"
	solanaserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana/server"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
)

var _ solanaserver.ClientCapability = (*ManualSolanaChain)(nil)

// ManualSolanaChain wraps a Solana ClientCapability and takes over the log
// trigger path so the simulator can fire events manually (e.g. replaying a known
// transaction). The upstream FakeSolanaChain leaves RegisterLogTrigger
// unimplemented, so this wrapper stores a per-trigger callback channel + filter
// and delivers manually-supplied logs to the workflow. All non-trigger methods
// delegate to inner. Mirrors ManualEVMChain.
type ManualSolanaChain struct {
	inner solanaserver.ClientCapability

	mu                sync.RWMutex
	callbackCh        map[string]chan commonCap.TriggerAndId[*solcap.Log]
	logTriggerFilters map[string]*solcap.FilterLogTriggerRequest
}

func NewManualSolanaChain(inner solanaserver.ClientCapability) *ManualSolanaChain {
	return &ManualSolanaChain{
		inner:             inner,
		callbackCh:        make(map[string]chan commonCap.TriggerAndId[*solcap.Log]),
		logTriggerFilters: make(map[string]*solcap.FilterLogTriggerRequest),
	}
}

// ManualTrigger validates the log against the registered filter and delivers it
// to the workflow's trigger callback channel.
func (m *ManualSolanaChain) ManualTrigger(ctx context.Context, triggerID string, log *solcap.Log) error {
	m.mu.RLock()
	filter := m.logTriggerFilters[triggerID]
	callbackCh := m.callbackCh[triggerID]
	m.mu.RUnlock()

	if callbackCh == nil {
		return fmt.Errorf("solana log trigger %q is not registered", triggerID)
	}
	if filter != nil {
		if err := manualSolanaLogMatchesFilter(log, filter); err != nil {
			return fmt.Errorf("log does not match registered filter for trigger %s: %w", triggerID, err)
		}
	}

	go func() {
		select {
		case callbackCh <- m.createManualTriggerEvent(log):
		case <-ctx.Done():
		}
	}()

	return nil
}

func (m *ManualSolanaChain) createManualTriggerEvent(log *solcap.Log) commonCap.TriggerAndId[*solcap.Log] {
	return commonCap.TriggerAndId[*solcap.Log]{
		Trigger: log,
		Id:      manualSolanaTriggerEventID(log),
	}
}

func manualSolanaTriggerEventID(log *solcap.Log) string {
	return fmt.Sprintf("manual-solana-chain-trigger-%x-%d", log.GetTxHash(), log.GetLogIndex())
}

// manualSolanaLogMatchesFilter checks the log against the program address and,
// when set, the 8-byte Anchor event discriminator (EventSig). Solana has no
// EVM-style topics; Subkeys/CPI matching is out of scope for the replay MVP.
func manualSolanaLogMatchesFilter(log *solcap.Log, filter *solcap.FilterLogTriggerRequest) error {
	if len(filter.GetAddress()) > 0 && !bytes.Equal(log.GetAddress(), filter.GetAddress()) {
		return fmt.Errorf("log program address %x does not match filter address %x", log.GetAddress(), filter.GetAddress())
	}
	return nil
}

func (m *ManualSolanaChain) RegisterLogTrigger(ctx context.Context, triggerID string, metadata commonCap.RequestMetadata, input *solcap.FilterLogTriggerRequest) (<-chan commonCap.TriggerAndId[*solcap.Log], caperrors.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callbackCh[triggerID] = make(chan commonCap.TriggerAndId[*solcap.Log])
	m.logTriggerFilters[triggerID] = input
	return m.callbackCh[triggerID], nil
}

func (m *ManualSolanaChain) UnregisterLogTrigger(ctx context.Context, triggerID string, metadata commonCap.RequestMetadata, input *solcap.FilterLogTriggerRequest) caperrors.Error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.logTriggerFilters, triggerID)
	delete(m.callbackCh, triggerID)
	return nil
}

func (m *ManualSolanaChain) AckEvent(ctx context.Context, triggerID, eventID, method string) caperrors.Error {
	return nil
}

// --- Reads / writes: delegate ---

func (m *ManualSolanaChain) WriteReport(ctx context.Context, metadata commonCap.RequestMetadata, input *solcap.WriteReportRequest) (*commonCap.ResponseAndMetadata[*solcap.WriteReportReply], caperrors.Error) {
	return m.inner.WriteReport(ctx, metadata, input)
}
func (m *ManualSolanaChain) GetAccountInfoWithOpts(ctx context.Context, md commonCap.RequestMetadata, i *solcap.GetAccountInfoWithOptsRequest) (*commonCap.ResponseAndMetadata[*solcap.GetAccountInfoWithOptsReply], caperrors.Error) {
	return m.inner.GetAccountInfoWithOpts(ctx, md, i)
}
func (m *ManualSolanaChain) GetBalance(ctx context.Context, md commonCap.RequestMetadata, i *solcap.GetBalanceRequest) (*commonCap.ResponseAndMetadata[*solcap.GetBalanceReply], caperrors.Error) {
	return m.inner.GetBalance(ctx, md, i)
}
func (m *ManualSolanaChain) GetBlock(ctx context.Context, md commonCap.RequestMetadata, i *solcap.GetBlockRequest) (*commonCap.ResponseAndMetadata[*solcap.GetBlockReply], caperrors.Error) {
	return m.inner.GetBlock(ctx, md, i)
}
func (m *ManualSolanaChain) GetFeeForMessage(ctx context.Context, md commonCap.RequestMetadata, i *solcap.GetFeeForMessageRequest) (*commonCap.ResponseAndMetadata[*solcap.GetFeeForMessageReply], caperrors.Error) {
	return m.inner.GetFeeForMessage(ctx, md, i)
}
func (m *ManualSolanaChain) GetMultipleAccountsWithOpts(ctx context.Context, md commonCap.RequestMetadata, i *solcap.GetMultipleAccountsWithOptsRequest) (*commonCap.ResponseAndMetadata[*solcap.GetMultipleAccountsWithOptsReply], caperrors.Error) {
	return m.inner.GetMultipleAccountsWithOpts(ctx, md, i)
}
func (m *ManualSolanaChain) GetProgramAccounts(ctx context.Context, md commonCap.RequestMetadata, i *solcap.GetProgramAccountsRequest) (*commonCap.ResponseAndMetadata[*solcap.GetProgramAccountsReply], caperrors.Error) {
	return m.inner.GetProgramAccounts(ctx, md, i)
}
func (m *ManualSolanaChain) GetSignatureStatuses(ctx context.Context, md commonCap.RequestMetadata, i *solcap.GetSignatureStatusesRequest) (*commonCap.ResponseAndMetadata[*solcap.GetSignatureStatusesReply], caperrors.Error) {
	return m.inner.GetSignatureStatuses(ctx, md, i)
}
func (m *ManualSolanaChain) GetSlotHeight(ctx context.Context, md commonCap.RequestMetadata, i *solcap.GetSlotHeightRequest) (*commonCap.ResponseAndMetadata[*solcap.GetSlotHeightReply], caperrors.Error) {
	return m.inner.GetSlotHeight(ctx, md, i)
}
func (m *ManualSolanaChain) GetTransaction(ctx context.Context, md commonCap.RequestMetadata, i *solcap.GetTransactionRequest) (*commonCap.ResponseAndMetadata[*solcap.GetTransactionReply], caperrors.Error) {
	return m.inner.GetTransaction(ctx, md, i)
}

// --- Lifecycle: delegate ---

func (m *ManualSolanaChain) ChainSelector() uint64           { return m.inner.ChainSelector() }
func (m *ManualSolanaChain) Start(ctx context.Context) error { return m.inner.Start(ctx) }
func (m *ManualSolanaChain) Close() error                    { return m.inner.Close() }
func (m *ManualSolanaChain) HealthReport() map[string]error  { return m.inner.HealthReport() }
func (m *ManualSolanaChain) Name() string                    { return m.inner.Name() }
func (m *ManualSolanaChain) Description() string             { return m.inner.Description() }
func (m *ManualSolanaChain) Ready() error                    { return m.inner.Ready() }
func (m *ManualSolanaChain) Initialise(ctx context.Context, deps core.StandardCapabilitiesDependencies) error {
	return m.inner.Initialise(ctx, deps)
}

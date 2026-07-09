package solana

import (
	"context"
	"fmt"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	solcap "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana"
	solanaserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana/server"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// LimitedSolanaChain enforces chain-write report size + Solana compute-unit limit.
type LimitedSolanaChain struct {
	inner  solanaserver.ClientCapability
	limits chain.Limits
}

var _ solanaserver.ClientCapability = (*LimitedSolanaChain)(nil)

func NewLimitedSolanaChain(inner solanaserver.ClientCapability, limits chain.Limits) *LimitedSolanaChain {
	return &LimitedSolanaChain{inner: inner, limits: limits}
}

func (l *LimitedSolanaChain) WriteReport(ctx context.Context, metadata commonCap.RequestMetadata, input *solcap.WriteReportRequest) (*commonCap.ResponseAndMetadata[*solcap.WriteReportReply], caperrors.Error) {
	if input != nil && input.Report != nil {
		if lim := l.limits.ReportSize; lim > 0 && len(input.Report.RawReport) > lim {
			return nil, caperrors.NewPublicUserError(
				fmt.Errorf("simulation limit exceeded: Solana chain write report size %d bytes exceeds limit of %d bytes", len(input.Report.RawReport), lim),
				caperrors.ResourceExhausted,
			)
		}
	}
	if input != nil && input.ComputeConfig != nil {
		if gl := l.limits.GasLimit; gl > 0 && uint64(input.ComputeConfig.ComputeLimit) > gl {
			return nil, caperrors.NewPublicUserError(
				fmt.Errorf("simulation limit exceeded: Solana compute_limit %d exceeds maximum of %d", input.ComputeConfig.ComputeLimit, gl),
				caperrors.ResourceExhausted,
			)
		}
	}
	return l.inner.WriteReport(ctx, metadata, input)
}

// --- Reads: delegate ---

func (l *LimitedSolanaChain) GetAccountInfoWithOpts(ctx context.Context, m commonCap.RequestMetadata, i *solcap.GetAccountInfoWithOptsRequest) (*commonCap.ResponseAndMetadata[*solcap.GetAccountInfoWithOptsReply], caperrors.Error) {
	return l.inner.GetAccountInfoWithOpts(ctx, m, i)
}
func (l *LimitedSolanaChain) GetBalance(ctx context.Context, m commonCap.RequestMetadata, i *solcap.GetBalanceRequest) (*commonCap.ResponseAndMetadata[*solcap.GetBalanceReply], caperrors.Error) {
	return l.inner.GetBalance(ctx, m, i)
}
func (l *LimitedSolanaChain) GetBlock(ctx context.Context, m commonCap.RequestMetadata, i *solcap.GetBlockRequest) (*commonCap.ResponseAndMetadata[*solcap.GetBlockReply], caperrors.Error) {
	return l.inner.GetBlock(ctx, m, i)
}
func (l *LimitedSolanaChain) GetFeeForMessage(ctx context.Context, m commonCap.RequestMetadata, i *solcap.GetFeeForMessageRequest) (*commonCap.ResponseAndMetadata[*solcap.GetFeeForMessageReply], caperrors.Error) {
	return l.inner.GetFeeForMessage(ctx, m, i)
}
func (l *LimitedSolanaChain) GetMultipleAccountsWithOpts(ctx context.Context, m commonCap.RequestMetadata, i *solcap.GetMultipleAccountsWithOptsRequest) (*commonCap.ResponseAndMetadata[*solcap.GetMultipleAccountsWithOptsReply], caperrors.Error) {
	return l.inner.GetMultipleAccountsWithOpts(ctx, m, i)
}
func (l *LimitedSolanaChain) GetProgramAccounts(ctx context.Context, m commonCap.RequestMetadata, i *solcap.GetProgramAccountsRequest) (*commonCap.ResponseAndMetadata[*solcap.GetProgramAccountsReply], caperrors.Error) {
	return l.inner.GetProgramAccounts(ctx, m, i)
}
func (l *LimitedSolanaChain) GetSignatureStatuses(ctx context.Context, m commonCap.RequestMetadata, i *solcap.GetSignatureStatusesRequest) (*commonCap.ResponseAndMetadata[*solcap.GetSignatureStatusesReply], caperrors.Error) {
	return l.inner.GetSignatureStatuses(ctx, m, i)
}
func (l *LimitedSolanaChain) GetSlotHeight(ctx context.Context, m commonCap.RequestMetadata, i *solcap.GetSlotHeightRequest) (*commonCap.ResponseAndMetadata[*solcap.GetSlotHeightReply], caperrors.Error) {
	return l.inner.GetSlotHeight(ctx, m, i)
}
func (l *LimitedSolanaChain) GetTransaction(ctx context.Context, m commonCap.RequestMetadata, i *solcap.GetTransactionRequest) (*commonCap.ResponseAndMetadata[*solcap.GetTransactionReply], caperrors.Error) {
	return l.inner.GetTransaction(ctx, m, i)
}

// --- Triggers: delegate ---

func (l *LimitedSolanaChain) RegisterLogTrigger(ctx context.Context, triggerID string, m commonCap.RequestMetadata, i *solcap.FilterLogTriggerRequest) (<-chan commonCap.TriggerAndId[*solcap.Log], caperrors.Error) {
	return l.inner.RegisterLogTrigger(ctx, triggerID, m, i)
}
func (l *LimitedSolanaChain) UnregisterLogTrigger(ctx context.Context, triggerID string, m commonCap.RequestMetadata, i *solcap.FilterLogTriggerRequest) caperrors.Error {
	return l.inner.UnregisterLogTrigger(ctx, triggerID, m, i)
}
func (l *LimitedSolanaChain) AckEvent(ctx context.Context, triggerID, eventID, method string) caperrors.Error {
	return l.inner.AckEvent(ctx, triggerID, eventID, method)
}

// --- Lifecycle: delegate ---

func (l *LimitedSolanaChain) ChainSelector() uint64           { return l.inner.ChainSelector() }
func (l *LimitedSolanaChain) Start(ctx context.Context) error { return l.inner.Start(ctx) }
func (l *LimitedSolanaChain) Close() error                    { return l.inner.Close() }
func (l *LimitedSolanaChain) HealthReport() map[string]error  { return l.inner.HealthReport() }
func (l *LimitedSolanaChain) Name() string                    { return l.inner.Name() }
func (l *LimitedSolanaChain) Description() string             { return l.inner.Description() }
func (l *LimitedSolanaChain) Ready() error                    { return l.inner.Ready() }
func (l *LimitedSolanaChain) Initialise(ctx context.Context, deps core.StandardCapabilitiesDependencies) error {
	return l.inner.Initialise(ctx, deps)
}

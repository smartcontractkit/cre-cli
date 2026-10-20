package stellar

import (
	"context"
	"fmt"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	stellarcap "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/stellar"
	stellarserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/stellar/server"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// LimitedStellarChain enforces the chain-write report size limit.
type LimitedStellarChain struct {
	inner  stellarserver.ClientCapability
	limits chain.Limits
}

var _ stellarserver.ClientCapability = (*LimitedStellarChain)(nil)

func NewLimitedStellarChain(inner stellarserver.ClientCapability, limits chain.Limits) *LimitedStellarChain {
	return &LimitedStellarChain{inner: inner, limits: limits}
}

func (l *LimitedStellarChain) WriteReport(ctx context.Context, metadata commonCap.RequestMetadata, input *stellarcap.WriteReportRequest) (*commonCap.ResponseAndMetadata[*stellarcap.WriteReportReply], caperrors.Error) {
	if input != nil && input.Report != nil {
		if lim := l.limits.ReportSize; lim > 0 && len(input.Report.RawReport) > lim {
			return nil, caperrors.NewPublicUserError(
				fmt.Errorf("simulation limit exceeded: Stellar chain write report size %d bytes exceeds limit of %d bytes", len(input.Report.RawReport), lim),
				caperrors.ResourceExhausted,
			)
		}
	}
	return l.inner.WriteReport(ctx, metadata, input)
}

// --- Reads: delegate ---

func (l *LimitedStellarChain) GetLatestLedger(ctx context.Context, m commonCap.RequestMetadata, i *stellarcap.GetLatestLedgerRequest) (*commonCap.ResponseAndMetadata[*stellarcap.GetLatestLedgerResponse], caperrors.Error) {
	return l.inner.GetLatestLedger(ctx, m, i)
}
func (l *LimitedStellarChain) ReadContract(ctx context.Context, m commonCap.RequestMetadata, i *stellarcap.ReadContractRequest) (*commonCap.ResponseAndMetadata[*stellarcap.ReadContractResponse], caperrors.Error) {
	return l.inner.ReadContract(ctx, m, i)
}

// --- Lifecycle: delegate ---

func (l *LimitedStellarChain) ChainSelector() uint64           { return l.inner.ChainSelector() }
func (l *LimitedStellarChain) Start(ctx context.Context) error { return l.inner.Start(ctx) }
func (l *LimitedStellarChain) Close() error                    { return l.inner.Close() }
func (l *LimitedStellarChain) HealthReport() map[string]error  { return l.inner.HealthReport() }
func (l *LimitedStellarChain) Name() string                    { return l.inner.Name() }
func (l *LimitedStellarChain) Description() string             { return l.inner.Description() }
func (l *LimitedStellarChain) Ready() error                    { return l.inner.Ready() }
func (l *LimitedStellarChain) Initialise(ctx context.Context, deps core.StandardCapabilitiesDependencies) error {
	return l.inner.Initialise(ctx, deps)
}

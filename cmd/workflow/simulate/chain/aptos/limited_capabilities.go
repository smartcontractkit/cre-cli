package aptos

import (
	"context"
	"fmt"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	aptoscappb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/aptos"
	aptosserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/aptos/server"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// LimitedAptosChain enforces chain-write size + Aptos max_gas_amount.
type LimitedAptosChain struct {
	inner  aptosserver.ClientCapability
	limits chain.Limits
}

var _ aptosserver.ClientCapability = (*LimitedAptosChain)(nil)

func NewLimitedAptosChain(inner aptosserver.ClientCapability, limits chain.Limits) *LimitedAptosChain {
	return &LimitedAptosChain{inner: inner, limits: limits}
}

func (l *LimitedAptosChain) WriteReport(ctx context.Context, metadata commonCap.RequestMetadata, input *aptoscappb.WriteReportRequest) (*commonCap.ResponseAndMetadata[*aptoscappb.WriteReportReply], caperrors.Error) {
	if input.Report != nil {
		if lim := l.limits.ReportSize; lim > 0 && len(input.Report.RawReport) > lim {
			return nil, caperrors.NewPublicUserError(
				fmt.Errorf("simulation limit exceeded: aptos report size %d > %d", len(input.Report.RawReport), lim),
				caperrors.ResourceExhausted,
			)
		}
	}
	if input.GasConfig != nil {
		if gl := l.limits.GasLimit; gl > 0 && input.GasConfig.MaxGasAmount > gl {
			return nil, caperrors.NewPublicUserError(
				fmt.Errorf("simulation limit exceeded: aptos max_gas_amount %d > %d", input.GasConfig.MaxGasAmount, gl),
				caperrors.ResourceExhausted,
			)
		}
	}
	return l.inner.WriteReport(ctx, metadata, input)
}

func (l *LimitedAptosChain) AccountAPTBalance(ctx context.Context, m commonCap.RequestMetadata, i *aptoscappb.AccountAPTBalanceRequest) (*commonCap.ResponseAndMetadata[*aptoscappb.AccountAPTBalanceReply], caperrors.Error) {
	return l.inner.AccountAPTBalance(ctx, m, i)
}
func (l *LimitedAptosChain) View(ctx context.Context, m commonCap.RequestMetadata, i *aptoscappb.ViewRequest) (*commonCap.ResponseAndMetadata[*aptoscappb.ViewReply], caperrors.Error) {
	return l.inner.View(ctx, m, i)
}
func (l *LimitedAptosChain) TransactionByHash(ctx context.Context, m commonCap.RequestMetadata, i *aptoscappb.TransactionByHashRequest) (*commonCap.ResponseAndMetadata[*aptoscappb.TransactionByHashReply], caperrors.Error) {
	return l.inner.TransactionByHash(ctx, m, i)
}
func (l *LimitedAptosChain) AccountTransactions(ctx context.Context, m commonCap.RequestMetadata, i *aptoscappb.AccountTransactionsRequest) (*commonCap.ResponseAndMetadata[*aptoscappb.AccountTransactionsReply], caperrors.Error) {
	return l.inner.AccountTransactions(ctx, m, i)
}

func (l *LimitedAptosChain) ChainSelector() uint64           { return l.inner.ChainSelector() }
func (l *LimitedAptosChain) Start(ctx context.Context) error { return l.inner.Start(ctx) }
func (l *LimitedAptosChain) Close() error                    { return l.inner.Close() }
func (l *LimitedAptosChain) HealthReport() map[string]error  { return l.inner.HealthReport() }
func (l *LimitedAptosChain) Name() string                    { return l.inner.Name() }
func (l *LimitedAptosChain) Description() string             { return l.inner.Description() }
func (l *LimitedAptosChain) Ready() error                    { return l.inner.Ready() }
func (l *LimitedAptosChain) Initialise(ctx context.Context, deps core.StandardCapabilitiesDependencies) error {
	return l.inner.Initialise(ctx, deps)
}

package evm

import (
	"context"
	"fmt"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	evmcappb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	evmserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm/server"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// EVMChainLimits is the limit-accessor contract LimitedEVMChain enforces.
// Aliased to chain.Limits so the family-agnostic CapabilityConfig.Limits
// value can be passed straight through without a type assertion.
type EVMChainLimits = chain.Limits

// LimitedEVMChain wraps an evmserver.ClientCapability and enforces chain write
// report size and gas limits.
type LimitedEVMChain struct {
	inner  evmserver.ClientCapability
	limits EVMChainLimits
}

var _ evmserver.ClientCapability = (*LimitedEVMChain)(nil)

func NewLimitedEVMChain(inner evmserver.ClientCapability, limits EVMChainLimits) *LimitedEVMChain {
	return &LimitedEVMChain{inner: inner, limits: limits}
}

func (l *LimitedEVMChain) WriteReport(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.WriteReportRequest) (*commonCap.ResponseAndMetadata[*evmcappb.WriteReportReply], caperrors.Error) {
	// Check report size
	reportLimit := l.limits.ChainWriteReportSizeLimit()
	if reportLimit > 0 && input.Report != nil && len(input.Report.RawReport) > reportLimit {
		return nil, caperrors.NewPublicUserError(
			fmt.Errorf("simulation limit exceeded: chain write report size %d bytes exceeds limit of %d bytes", len(input.Report.RawReport), reportLimit),
			caperrors.ResourceExhausted,
		)
	}

	// Check gas limit
	gasLimit := l.limits.ChainWriteEVMGasLimit()
	if gasLimit > 0 && input.GasConfig != nil && input.GasConfig.GasLimit > gasLimit {
		return nil, caperrors.NewPublicUserError(
			fmt.Errorf("simulation limit exceeded: EVM gas limit %d exceeds maximum of %d", input.GasConfig.GasLimit, gasLimit),
			caperrors.ResourceExhausted,
		)
	}

	return l.inner.WriteReport(ctx, metadata, input)
}

// All other methods delegate to the inner capability.

func (l *LimitedEVMChain) CallContract(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.CallContractRequest) (*commonCap.ResponseAndMetadata[*evmcappb.CallContractReply], caperrors.Error) {
	return l.inner.CallContract(ctx, metadata, input)
}

func (l *LimitedEVMChain) FilterLogs(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.FilterLogsRequest) (*commonCap.ResponseAndMetadata[*evmcappb.FilterLogsReply], caperrors.Error) {
	return l.inner.FilterLogs(ctx, metadata, input)
}

func (l *LimitedEVMChain) BalanceAt(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.BalanceAtRequest) (*commonCap.ResponseAndMetadata[*evmcappb.BalanceAtReply], caperrors.Error) {
	return l.inner.BalanceAt(ctx, metadata, input)
}

func (l *LimitedEVMChain) EstimateGas(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.EstimateGasRequest) (*commonCap.ResponseAndMetadata[*evmcappb.EstimateGasReply], caperrors.Error) {
	return l.inner.EstimateGas(ctx, metadata, input)
}

func (l *LimitedEVMChain) GetTransactionByHash(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.GetTransactionByHashRequest) (*commonCap.ResponseAndMetadata[*evmcappb.GetTransactionByHashReply], caperrors.Error) {
	return l.inner.GetTransactionByHash(ctx, metadata, input)
}

func (l *LimitedEVMChain) GetTransactionReceipt(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.GetTransactionReceiptRequest) (*commonCap.ResponseAndMetadata[*evmcappb.GetTransactionReceiptReply], caperrors.Error) {
	return l.inner.GetTransactionReceipt(ctx, metadata, input)
}

func (l *LimitedEVMChain) HeaderByNumber(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.HeaderByNumberRequest) (*commonCap.ResponseAndMetadata[*evmcappb.HeaderByNumberReply], caperrors.Error) {
	return l.inner.HeaderByNumber(ctx, metadata, input)
}

func (l *LimitedEVMChain) RegisterLogTrigger(ctx context.Context, triggerID string, metadata commonCap.RequestMetadata, input *evmcappb.FilterLogTriggerRequest) (<-chan commonCap.TriggerAndId[*evmcappb.Log], caperrors.Error) {
	return l.inner.RegisterLogTrigger(ctx, triggerID, metadata, input)
}

func (l *LimitedEVMChain) UnregisterLogTrigger(ctx context.Context, triggerID string, metadata commonCap.RequestMetadata, input *evmcappb.FilterLogTriggerRequest) caperrors.Error {
	return l.inner.UnregisterLogTrigger(ctx, triggerID, metadata, input)
}

func (l *LimitedEVMChain) ChainSelector() uint64           { return l.inner.ChainSelector() }
func (l *LimitedEVMChain) Start(ctx context.Context) error { return l.inner.Start(ctx) }
func (l *LimitedEVMChain) Close() error                    { return l.inner.Close() }
func (l *LimitedEVMChain) HealthReport() map[string]error  { return l.inner.HealthReport() }
func (l *LimitedEVMChain) Name() string                    { return l.inner.Name() }
func (l *LimitedEVMChain) Description() string             { return l.inner.Description() }
func (l *LimitedEVMChain) Ready() error                    { return l.inner.Ready() }
func (l *LimitedEVMChain) Initialise(ctx context.Context, deps core.StandardCapabilitiesDependencies) error {
	return l.inner.Initialise(ctx, deps)
}

func (l *LimitedEVMChain) AckEvent(ctx context.Context, triggerId string, eventId string, method string) caperrors.Error {
	return l.inner.AckEvent(ctx, triggerId, eventId, method)
}

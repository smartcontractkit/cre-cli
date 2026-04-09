package evm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	evmcappb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	evmserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm/server"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
	sdkpb "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
)

type EVMChainLimit struct {
	reportSizeLimit int
	gasLimit        uint64
}

func (s *EVMChainLimit) ChainWriteReportSizeLimit() int { return s.reportSizeLimit }
func (s *EVMChainLimit) ChainWriteEVMGasLimit() uint64  { return s.gasLimit }

type evmCapabilityBaseStub struct{}

func (evmCapabilityBaseStub) Start(context.Context) error                                    { return nil }
func (evmCapabilityBaseStub) Close() error                                                   { return nil }
func (evmCapabilityBaseStub) HealthReport() map[string]error                                 { return map[string]error{} }
func (evmCapabilityBaseStub) Name() string                                                   { return "stub" }
func (evmCapabilityBaseStub) Description() string                                            { return "stub" }
func (evmCapabilityBaseStub) Ready() error                                                   { return nil }
func (evmCapabilityBaseStub) Initialise(context.Context, core.StandardCapabilitiesDependencies) error { return nil }

type evmClientCapabilityStub struct {
	evmCapabilityBaseStub
	writeReportFn    func(context.Context, commonCap.RequestMetadata, *evmcappb.WriteReportRequest) (*commonCap.ResponseAndMetadata[*evmcappb.WriteReportReply], caperrors.Error)
	writeReportCalls int
}

var _ evmserver.ClientCapability = (*evmClientCapabilityStub)(nil)

func (s *evmClientCapabilityStub) CallContract(context.Context, commonCap.RequestMetadata, *evmcappb.CallContractRequest) (*commonCap.ResponseAndMetadata[*evmcappb.CallContractReply], caperrors.Error) {
	return nil, nil
}

func (s *evmClientCapabilityStub) FilterLogs(context.Context, commonCap.RequestMetadata, *evmcappb.FilterLogsRequest) (*commonCap.ResponseAndMetadata[*evmcappb.FilterLogsReply], caperrors.Error) {
	return nil, nil
}

func (s *evmClientCapabilityStub) BalanceAt(context.Context, commonCap.RequestMetadata, *evmcappb.BalanceAtRequest) (*commonCap.ResponseAndMetadata[*evmcappb.BalanceAtReply], caperrors.Error) {
	return nil, nil
}

func (s *evmClientCapabilityStub) EstimateGas(context.Context, commonCap.RequestMetadata, *evmcappb.EstimateGasRequest) (*commonCap.ResponseAndMetadata[*evmcappb.EstimateGasReply], caperrors.Error) {
	return nil, nil
}

func (s *evmClientCapabilityStub) GetTransactionByHash(context.Context, commonCap.RequestMetadata, *evmcappb.GetTransactionByHashRequest) (*commonCap.ResponseAndMetadata[*evmcappb.GetTransactionByHashReply], caperrors.Error) {
	return nil, nil
}

func (s *evmClientCapabilityStub) GetTransactionReceipt(context.Context, commonCap.RequestMetadata, *evmcappb.GetTransactionReceiptRequest) (*commonCap.ResponseAndMetadata[*evmcappb.GetTransactionReceiptReply], caperrors.Error) {
	return nil, nil
}

func (s *evmClientCapabilityStub) HeaderByNumber(context.Context, commonCap.RequestMetadata, *evmcappb.HeaderByNumberRequest) (*commonCap.ResponseAndMetadata[*evmcappb.HeaderByNumberReply], caperrors.Error) {
	return nil, nil
}

func (s *evmClientCapabilityStub) RegisterLogTrigger(context.Context, string, commonCap.RequestMetadata, *evmcappb.FilterLogTriggerRequest) (<-chan commonCap.TriggerAndId[*evmcappb.Log], caperrors.Error) {
	return nil, nil
}

func (s *evmClientCapabilityStub) UnregisterLogTrigger(context.Context, string, commonCap.RequestMetadata, *evmcappb.FilterLogTriggerRequest) caperrors.Error {
	return nil
}

func (s *evmClientCapabilityStub) WriteReport(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.WriteReportRequest) (*commonCap.ResponseAndMetadata[*evmcappb.WriteReportReply], caperrors.Error) {
	s.writeReportCalls++
	if s.writeReportFn != nil {
		return s.writeReportFn(ctx, metadata, input)
	}
	return nil, nil
}
func (s *evmClientCapabilityStub) AckEvent(context.Context, string, string, string) caperrors.Error {
	return nil
}
func (s *evmClientCapabilityStub) ChainSelector() uint64 { return 0 }

func TestLimitedEVMChainWriteReportRejectsOversizedReport(t *testing.T) {
	t.Parallel()

	limits := &EVMChainLimit{reportSizeLimit: 4}
	inner := &evmClientCapabilityStub{}
	wrapper := NewLimitedEVMChain(inner, limits)

	resp, err := wrapper.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{
		Report: &sdkpb.ReportResponse{RawReport: []byte("12345")},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "chain write report size 5 bytes exceeds limit of 4 bytes")
	assert.Equal(t, 0, inner.writeReportCalls)
}

func TestLimitedEVMChainWriteReportRejectsOversizedGasLimit(t *testing.T) {
	t.Parallel()

	limits := &EVMChainLimit{gasLimit: 10}
	inner := &evmClientCapabilityStub{}
	wrapper := NewLimitedEVMChain(inner, limits)

	resp, err := wrapper.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{
		GasConfig: &evmcappb.GasConfig{GasLimit: 11},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "EVM gas limit 11 exceeds maximum of 10")
	assert.Equal(t, 0, inner.writeReportCalls)
}

func TestLimitedEVMChainWriteReportDelegatesOnBoundaryValues(t *testing.T) {
	t.Parallel()

	limits := &EVMChainLimit{reportSizeLimit: 4, gasLimit: 10}

	input := &evmcappb.WriteReportRequest{
		Report:    &sdkpb.ReportResponse{RawReport: []byte("1234")},
		GasConfig: &evmcappb.GasConfig{GasLimit: 10},
	}
	expectedResp := &commonCap.ResponseAndMetadata[*evmcappb.WriteReportReply]{Response: &evmcappb.WriteReportReply{}}

	inner := &evmClientCapabilityStub{
		writeReportFn: func(_ context.Context, _ commonCap.RequestMetadata, got *evmcappb.WriteReportRequest) (*commonCap.ResponseAndMetadata[*evmcappb.WriteReportReply], caperrors.Error) {
			assert.Same(t, input, got)
			return expectedResp, nil
		},
	}

	wrapper := NewLimitedEVMChain(inner, limits)
	resp, err := wrapper.WriteReport(context.Background(), commonCap.RequestMetadata{}, input)
	require.NoError(t, err)
	assert.Same(t, expectedResp, resp)
	assert.Equal(t, 1, inner.writeReportCalls)
}

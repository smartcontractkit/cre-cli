package aptos

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	aptoscappb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/aptos"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
	sdk "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
)

type stubLimits struct {
	reportSize int
	maxGas     uint64
}

func (s stubLimits) ChainWriteReportSizeLimit() int      { return s.reportSize }
func (s stubLimits) ChainWriteAptosMaxGasAmount() uint64 { return s.maxGas }

type stubCap struct{ writeCalled bool }

func (s *stubCap) AccountAPTBalance(context.Context, commonCap.RequestMetadata, *aptoscappb.AccountAPTBalanceRequest) (*commonCap.ResponseAndMetadata[*aptoscappb.AccountAPTBalanceReply], caperrors.Error) {
	return nil, nil
}
func (s *stubCap) View(context.Context, commonCap.RequestMetadata, *aptoscappb.ViewRequest) (*commonCap.ResponseAndMetadata[*aptoscappb.ViewReply], caperrors.Error) {
	return nil, nil
}
func (s *stubCap) TransactionByHash(context.Context, commonCap.RequestMetadata, *aptoscappb.TransactionByHashRequest) (*commonCap.ResponseAndMetadata[*aptoscappb.TransactionByHashReply], caperrors.Error) {
	return nil, nil
}
func (s *stubCap) AccountTransactions(context.Context, commonCap.RequestMetadata, *aptoscappb.AccountTransactionsRequest) (*commonCap.ResponseAndMetadata[*aptoscappb.AccountTransactionsReply], caperrors.Error) {
	return nil, nil
}
func (s *stubCap) WriteReport(context.Context, commonCap.RequestMetadata, *aptoscappb.WriteReportRequest) (*commonCap.ResponseAndMetadata[*aptoscappb.WriteReportReply], caperrors.Error) {
	s.writeCalled = true
	return &commonCap.ResponseAndMetadata[*aptoscappb.WriteReportReply]{Response: &aptoscappb.WriteReportReply{}}, nil
}
func (s *stubCap) ChainSelector() uint64          { return 0 }
func (s *stubCap) Start(context.Context) error    { return nil }
func (s *stubCap) Close() error                   { return nil }
func (s *stubCap) HealthReport() map[string]error { return nil }
func (s *stubCap) Name() string                   { return "stub" }
func (s *stubCap) Description() string            { return "" }
func (s *stubCap) Ready() error                   { return nil }
func (s *stubCap) Initialise(context.Context, core.StandardCapabilitiesDependencies) error {
	return nil
}

func TestLimitedAptosChain_WriteReport_ReportTooLarge(t *testing.T) {
	t.Parallel()
	inner := &stubCap{}
	l := NewLimitedAptosChain(inner, stubLimits{reportSize: 10, maxGas: 1000})
	_, capErr := l.WriteReport(context.Background(), commonCap.RequestMetadata{}, &aptoscappb.WriteReportRequest{
		Report: &sdk.ReportResponse{RawReport: make([]byte, 11)},
	})
	require.NotNil(t, capErr)
	assert.False(t, inner.writeCalled)
}

func TestLimitedAptosChain_WriteReport_MaxGasTooHigh(t *testing.T) {
	t.Parallel()
	inner := &stubCap{}
	l := NewLimitedAptosChain(inner, stubLimits{reportSize: 100, maxGas: 100})
	_, capErr := l.WriteReport(context.Background(), commonCap.RequestMetadata{}, &aptoscappb.WriteReportRequest{
		Report:    &sdk.ReportResponse{RawReport: []byte("x")},
		GasConfig: &aptoscappb.GasConfig{MaxGasAmount: 101},
	})
	require.NotNil(t, capErr)
	assert.False(t, inner.writeCalled)
}

func TestLimitedAptosChain_WriteReport_Delegates(t *testing.T) {
	t.Parallel()
	inner := &stubCap{}
	l := NewLimitedAptosChain(inner, stubLimits{reportSize: 100, maxGas: 1000})
	_, capErr := l.WriteReport(context.Background(), commonCap.RequestMetadata{}, &aptoscappb.WriteReportRequest{
		Report:    &sdk.ReportResponse{RawReport: []byte("x")},
		GasConfig: &aptoscappb.GasConfig{MaxGasAmount: 50},
	})
	require.Nil(t, capErr)
	assert.True(t, inner.writeCalled)
}

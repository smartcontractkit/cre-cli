package evm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	evmcappb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
	sdkpb "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
)

// fullStubCapability extends the base stub with counters on every delegating
// method so we can verify the limiter passes calls through.
type fullStubCapability struct {
	evmCapabilityBaseStub
	calls map[string]int

	// optional return override
	writeErr caperrors.Error

	closeErr error
	startErr error
}

func newFullStub() *fullStubCapability {
	return &fullStubCapability{calls: map[string]int{}}
}

func (s *fullStubCapability) CallContract(context.Context, commonCap.RequestMetadata, *evmcappb.CallContractRequest) (*commonCap.ResponseAndMetadata[*evmcappb.CallContractReply], caperrors.Error) {
	s.calls["CallContract"]++
	return &commonCap.ResponseAndMetadata[*evmcappb.CallContractReply]{}, nil
}

func (s *fullStubCapability) FilterLogs(context.Context, commonCap.RequestMetadata, *evmcappb.FilterLogsRequest) (*commonCap.ResponseAndMetadata[*evmcappb.FilterLogsReply], caperrors.Error) {
	s.calls["FilterLogs"]++
	return &commonCap.ResponseAndMetadata[*evmcappb.FilterLogsReply]{}, nil
}

func (s *fullStubCapability) BalanceAt(context.Context, commonCap.RequestMetadata, *evmcappb.BalanceAtRequest) (*commonCap.ResponseAndMetadata[*evmcappb.BalanceAtReply], caperrors.Error) {
	s.calls["BalanceAt"]++
	return &commonCap.ResponseAndMetadata[*evmcappb.BalanceAtReply]{}, nil
}

func (s *fullStubCapability) EstimateGas(context.Context, commonCap.RequestMetadata, *evmcappb.EstimateGasRequest) (*commonCap.ResponseAndMetadata[*evmcappb.EstimateGasReply], caperrors.Error) {
	s.calls["EstimateGas"]++
	return &commonCap.ResponseAndMetadata[*evmcappb.EstimateGasReply]{}, nil
}

func (s *fullStubCapability) GetTransactionByHash(context.Context, commonCap.RequestMetadata, *evmcappb.GetTransactionByHashRequest) (*commonCap.ResponseAndMetadata[*evmcappb.GetTransactionByHashReply], caperrors.Error) {
	s.calls["GetTransactionByHash"]++
	return &commonCap.ResponseAndMetadata[*evmcappb.GetTransactionByHashReply]{}, nil
}

func (s *fullStubCapability) GetTransactionReceipt(context.Context, commonCap.RequestMetadata, *evmcappb.GetTransactionReceiptRequest) (*commonCap.ResponseAndMetadata[*evmcappb.GetTransactionReceiptReply], caperrors.Error) {
	s.calls["GetTransactionReceipt"]++
	return &commonCap.ResponseAndMetadata[*evmcappb.GetTransactionReceiptReply]{}, nil
}

func (s *fullStubCapability) HeaderByNumber(context.Context, commonCap.RequestMetadata, *evmcappb.HeaderByNumberRequest) (*commonCap.ResponseAndMetadata[*evmcappb.HeaderByNumberReply], caperrors.Error) {
	s.calls["HeaderByNumber"]++
	return &commonCap.ResponseAndMetadata[*evmcappb.HeaderByNumberReply]{}, nil
}

func (s *fullStubCapability) RegisterLogTrigger(context.Context, string, commonCap.RequestMetadata, *evmcappb.FilterLogTriggerRequest) (<-chan commonCap.TriggerAndId[*evmcappb.Log], caperrors.Error) {
	s.calls["RegisterLogTrigger"]++
	return nil, nil
}

func (s *fullStubCapability) UnregisterLogTrigger(context.Context, string, commonCap.RequestMetadata, *evmcappb.FilterLogTriggerRequest) caperrors.Error {
	s.calls["UnregisterLogTrigger"]++
	return nil
}

func (s *fullStubCapability) WriteReport(context.Context, commonCap.RequestMetadata, *evmcappb.WriteReportRequest) (*commonCap.ResponseAndMetadata[*evmcappb.WriteReportReply], caperrors.Error) {
	s.calls["WriteReport"]++
	if s.writeErr != nil {
		return nil, s.writeErr
	}
	return &commonCap.ResponseAndMetadata[*evmcappb.WriteReportReply]{}, nil
}

func (s *fullStubCapability) AckEvent(context.Context, string, string, string) caperrors.Error {
	s.calls["AckEvent"]++
	return nil
}

func (s *fullStubCapability) ChainSelector() uint64 {
	s.calls["ChainSelector"]++
	return 42
}

// Override lifecycle / metadata to count calls.
func (s *fullStubCapability) Start(context.Context) error {
	s.calls["Start"]++
	return s.startErr
}

func (s *fullStubCapability) Close() error {
	s.calls["Close"]++
	return s.closeErr
}

func (s *fullStubCapability) HealthReport() map[string]error {
	s.calls["HealthReport"]++
	return map[string]error{"ok": nil}
}

func (s *fullStubCapability) Name() string {
	s.calls["Name"]++
	return "stub-chain"
}

func (s *fullStubCapability) Description() string {
	s.calls["Description"]++
	return "stub-desc"
}

func (s *fullStubCapability) Ready() error {
	s.calls["Ready"]++
	return nil
}

func (s *fullStubCapability) Initialise(context.Context, core.StandardCapabilitiesDependencies) error {
	s.calls["Initialise"]++
	return nil
}

// ---------------------------------------------------------------------------
// Table-driven: every method is called through the limiter exactly once.
// ---------------------------------------------------------------------------

func TestLimitedEVMChain_AllMethodsDelegate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		call  func(w *LimitedEVMChain, s *fullStubCapability)
		wants string
	}{
		{"CallContract", func(w *LimitedEVMChain, _ *fullStubCapability) {
			_, _ = w.CallContract(context.Background(), commonCap.RequestMetadata{}, &evmcappb.CallContractRequest{})
		}, "CallContract"},
		{"FilterLogs", func(w *LimitedEVMChain, _ *fullStubCapability) {
			_, _ = w.FilterLogs(context.Background(), commonCap.RequestMetadata{}, &evmcappb.FilterLogsRequest{})
		}, "FilterLogs"},
		{"BalanceAt", func(w *LimitedEVMChain, _ *fullStubCapability) {
			_, _ = w.BalanceAt(context.Background(), commonCap.RequestMetadata{}, &evmcappb.BalanceAtRequest{})
		}, "BalanceAt"},
		{"EstimateGas", func(w *LimitedEVMChain, _ *fullStubCapability) {
			_, _ = w.EstimateGas(context.Background(), commonCap.RequestMetadata{}, &evmcappb.EstimateGasRequest{})
		}, "EstimateGas"},
		{"GetTransactionByHash", func(w *LimitedEVMChain, _ *fullStubCapability) {
			_, _ = w.GetTransactionByHash(context.Background(), commonCap.RequestMetadata{}, &evmcappb.GetTransactionByHashRequest{})
		}, "GetTransactionByHash"},
		{"GetTransactionReceipt", func(w *LimitedEVMChain, _ *fullStubCapability) {
			_, _ = w.GetTransactionReceipt(context.Background(), commonCap.RequestMetadata{}, &evmcappb.GetTransactionReceiptRequest{})
		}, "GetTransactionReceipt"},
		{"HeaderByNumber", func(w *LimitedEVMChain, _ *fullStubCapability) {
			_, _ = w.HeaderByNumber(context.Background(), commonCap.RequestMetadata{}, &evmcappb.HeaderByNumberRequest{})
		}, "HeaderByNumber"},
		{"RegisterLogTrigger", func(w *LimitedEVMChain, _ *fullStubCapability) {
			_, _ = w.RegisterLogTrigger(context.Background(), "tid", commonCap.RequestMetadata{}, &evmcappb.FilterLogTriggerRequest{})
		}, "RegisterLogTrigger"},
		{"UnregisterLogTrigger", func(w *LimitedEVMChain, _ *fullStubCapability) {
			_ = w.UnregisterLogTrigger(context.Background(), "tid", commonCap.RequestMetadata{}, &evmcappb.FilterLogTriggerRequest{})
		}, "UnregisterLogTrigger"},
		{"AckEvent", func(w *LimitedEVMChain, _ *fullStubCapability) {
			_ = w.AckEvent(context.Background(), "tid", "eid", "m")
		}, "AckEvent"},
		{"ChainSelector", func(w *LimitedEVMChain, s *fullStubCapability) {
			require.Equal(t, uint64(42), w.ChainSelector())
			require.Equal(t, 1, s.calls["ChainSelector"])
		}, "ChainSelector"},
		{"Start", func(w *LimitedEVMChain, _ *fullStubCapability) { _ = w.Start(context.Background()) }, "Start"},
		{"Close", func(w *LimitedEVMChain, _ *fullStubCapability) { _ = w.Close() }, "Close"},
		{"HealthReport", func(w *LimitedEVMChain, _ *fullStubCapability) { _ = w.HealthReport() }, "HealthReport"},
		{"Name", func(w *LimitedEVMChain, _ *fullStubCapability) { _ = w.Name() }, "Name"},
		{"Description", func(w *LimitedEVMChain, _ *fullStubCapability) { _ = w.Description() }, "Description"},
		{"Ready", func(w *LimitedEVMChain, _ *fullStubCapability) { _ = w.Ready() }, "Ready"},
		{"Initialise", func(w *LimitedEVMChain, _ *fullStubCapability) {
			_ = w.Initialise(context.Background(), core.StandardCapabilitiesDependencies{})
		}, "Initialise"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stub := newFullStub()
			w := NewLimitedEVMChain(stub, &EVMChainLimit{})
			tt.call(w, stub)
			assert.Equal(t, 1, stub.calls[tt.wants], "expected 1 call to %s", tt.wants)
		})
	}
}

// ---------------------------------------------------------------------------
// WriteReport policy edge cases
// ---------------------------------------------------------------------------

func TestLimitedEVMChain_WriteReport_NilReport_Delegates(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{reportSizeLimit: 4})
	resp, err := w.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{Report: nil})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, stub.calls["WriteReport"])
}

func TestLimitedEVMChain_WriteReport_NilGasConfig_Delegates(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{gasLimit: 100})
	resp, err := w.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{GasConfig: nil})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, stub.calls["WriteReport"])
}

func TestLimitedEVMChain_WriteReport_ZeroReportLimit_Disabled(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{reportSizeLimit: 0})
	resp, err := w.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{
		Report: &sdkpb.ReportResponse{RawReport: make([]byte, 1<<20)},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, stub.calls["WriteReport"])
}

func TestLimitedEVMChain_WriteReport_ZeroGasLimit_Disabled(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{gasLimit: 0})
	resp, err := w.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{
		GasConfig: &evmcappb.GasConfig{GasLimit: 1 << 30},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, stub.calls["WriteReport"])
}

func TestLimitedEVMChain_WriteReport_GasBoundaryEqualsLimit_Delegates(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{gasLimit: 1000})
	resp, err := w.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{
		GasConfig: &evmcappb.GasConfig{GasLimit: 1000},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestLimitedEVMChain_WriteReport_ReportBoundaryEqualsLimit_Delegates(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{reportSizeLimit: 5})
	resp, err := w.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{
		Report: &sdkpb.ReportResponse{RawReport: []byte("12345")},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestLimitedEVMChain_WriteReport_ReportOneOverLimit_Rejects(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{reportSizeLimit: 5})
	_, err := w.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{
		Report: &sdkpb.ReportResponse{RawReport: []byte("123456")},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "6 bytes exceeds limit of 5 bytes")
	assert.Equal(t, 0, stub.calls["WriteReport"])
}

func TestLimitedEVMChain_WriteReport_GasOneOverLimit_Rejects(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{gasLimit: 1_000_000})
	_, err := w.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{
		GasConfig: &evmcappb.GasConfig{GasLimit: 1_000_001},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1000001 exceeds maximum of 1000000")
	assert.Equal(t, 0, stub.calls["WriteReport"])
}

func TestLimitedEVMChain_WriteReport_ReportCheckedBeforeGas(t *testing.T) {
	// When both fail, report-size error is surfaced first.
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{reportSizeLimit: 1, gasLimit: 1})
	_, err := w.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{
		Report:    &sdkpb.ReportResponse{RawReport: []byte("ab")},
		GasConfig: &evmcappb.GasConfig{GasLimit: 999},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chain write report size")
	assert.NotContains(t, err.Error(), "EVM gas limit")
}

func TestLimitedEVMChain_WriteReport_ReturnsResourceExhaustedCode(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{reportSizeLimit: 1})
	_, err := w.WriteReport(context.Background(), commonCap.RequestMetadata{}, &evmcappb.WriteReportRequest{
		Report: &sdkpb.ReportResponse{RawReport: []byte("too-big")},
	})
	require.Error(t, err)
	assert.Equal(t, caperrors.ResourceExhausted, err.Code())
}

func TestLimitedEVMChain_Constructor_StoresInnerAndLimits(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	limits := &EVMChainLimit{reportSizeLimit: 7, gasLimit: 11}
	w := NewLimitedEVMChain(stub, limits)
	require.NotNil(t, w)
	// Verify Description delegates (indirectly proves inner is stored)
	require.Equal(t, "stub-desc", w.Description())
}

func TestLimitedEVMChain_ChainSelector_ReflectsInner(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{})
	require.Equal(t, uint64(42), w.ChainSelector())
}

func TestLimitedEVMChain_Name_ReflectsInner(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{})
	require.Equal(t, "stub-chain", w.Name())
}

func TestLimitedEVMChain_HealthReport_ReflectsInner(t *testing.T) {
	t.Parallel()
	stub := newFullStub()
	w := NewLimitedEVMChain(stub, &EVMChainLimit{})
	hr := w.HealthReport()
	_, ok := hr["ok"]
	require.True(t, ok)
}

package simulate

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/confidentialhttp"
	confhttpserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/confidentialhttp/server"
	customhttp "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/http"
	httpserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/http/server"
	evmcappb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	evmserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm/server"
	consensusserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/consensus/server"
	sdkpb "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
)

// --- LimitedHTTPAction ---

// LimitedHTTPAction wraps an httpserver.ClientCapability and enforces request/response
// size limits and connection timeout from SimulationLimits.
type LimitedHTTPAction struct {
	inner  httpserver.ClientCapability
	limits *SimulationLimits
}

var _ httpserver.ClientCapability = (*LimitedHTTPAction)(nil)

func NewLimitedHTTPAction(inner httpserver.ClientCapability, limits *SimulationLimits) *LimitedHTTPAction {
	return &LimitedHTTPAction{inner: inner, limits: limits}
}

func (l *LimitedHTTPAction) SendRequest(ctx context.Context, metadata commonCap.RequestMetadata, input *customhttp.Request) (*commonCap.ResponseAndMetadata[*customhttp.Response], caperrors.Error) {
	// Check request body size
	reqLimit := l.limits.HTTPRequestSizeLimit()
	if reqLimit > 0 && len(input.GetBody()) > reqLimit {
		return nil, caperrors.NewPublicUserError(
			fmt.Errorf("simulation limit exceeded: HTTP request body size %d bytes exceeds limit of %d bytes", len(input.GetBody()), reqLimit),
			caperrors.ResourceExhausted,
		)
	}

	// Enforce connection timeout
	connTimeout := l.limits.Workflows.HTTPAction.ConnectionTimeout.DefaultValue
	if connTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(connTimeout))
		defer cancel()
	}

	// Delegate to inner
	resp, capErr := l.inner.SendRequest(ctx, metadata, input)
	if capErr != nil {
		return resp, capErr
	}

	// Check response body size
	respLimit := l.limits.HTTPResponseSizeLimit()
	if resp != nil && resp.Response != nil && respLimit > 0 && len(resp.Response.GetBody()) > respLimit {
		return nil, caperrors.NewPublicUserError(
			fmt.Errorf("simulation limit exceeded: HTTP response body size %d bytes exceeds limit of %d bytes", len(resp.Response.GetBody()), respLimit),
			caperrors.ResourceExhausted,
		)
	}

	return resp, nil
}

func (l *LimitedHTTPAction) Start(ctx context.Context) error                           { return l.inner.Start(ctx) }
func (l *LimitedHTTPAction) Close() error                                              { return l.inner.Close() }
func (l *LimitedHTTPAction) HealthReport() map[string]error                            { return l.inner.HealthReport() }
func (l *LimitedHTTPAction) Name() string                                              { return l.inner.Name() }
func (l *LimitedHTTPAction) Description() string                                       { return l.inner.Description() }
func (l *LimitedHTTPAction) Ready() error                                              { return l.inner.Ready() }
func (l *LimitedHTTPAction) Initialise(ctx context.Context, deps core.StandardCapabilitiesDependencies) error {
	return l.inner.Initialise(ctx, deps)
}

// --- LimitedConfidentialHTTPAction ---

// LimitedConfidentialHTTPAction wraps a confhttpserver.ClientCapability and enforces
// request/response size limits and connection timeout from SimulationLimits.
type LimitedConfidentialHTTPAction struct {
	inner  confhttpserver.ClientCapability
	limits *SimulationLimits
}

var _ confhttpserver.ClientCapability = (*LimitedConfidentialHTTPAction)(nil)

func NewLimitedConfidentialHTTPAction(inner confhttpserver.ClientCapability, limits *SimulationLimits) *LimitedConfidentialHTTPAction {
	return &LimitedConfidentialHTTPAction{inner: inner, limits: limits}
}

func (l *LimitedConfidentialHTTPAction) SendRequest(ctx context.Context, metadata commonCap.RequestMetadata, input *confidentialhttp.ConfidentialHTTPRequest) (*commonCap.ResponseAndMetadata[*confidentialhttp.HTTPResponse], caperrors.Error) {
	// Check request size (body string or body bytes)
	reqLimit := l.limits.ConfHTTPRequestSizeLimit()
	if reqLimit > 0 && input.GetRequest() != nil {
		reqSize := len(input.GetRequest().GetBodyString()) + len(input.GetRequest().GetBodyBytes())
		if reqSize > reqLimit {
			return nil, caperrors.NewPublicUserError(
				fmt.Errorf("simulation limit exceeded: confidential HTTP request body size %d bytes exceeds limit of %d bytes", reqSize, reqLimit),
				caperrors.ResourceExhausted,
			)
		}
	}

	// Enforce connection timeout
	connTimeout := l.limits.Workflows.ConfidentialHTTP.ConnectionTimeout.DefaultValue
	if connTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(connTimeout))
		defer cancel()
	}

	// Delegate to inner
	resp, capErr := l.inner.SendRequest(ctx, metadata, input)
	if capErr != nil {
		return resp, capErr
	}

	// Check response body size
	respLimit := l.limits.ConfHTTPResponseSizeLimit()
	if resp != nil && resp.Response != nil && respLimit > 0 && len(resp.Response.GetBody()) > respLimit {
		return nil, caperrors.NewPublicUserError(
			fmt.Errorf("simulation limit exceeded: confidential HTTP response body size %d bytes exceeds limit of %d bytes", len(resp.Response.GetBody()), respLimit),
			caperrors.ResourceExhausted,
		)
	}

	return resp, nil
}

func (l *LimitedConfidentialHTTPAction) Start(ctx context.Context) error        { return l.inner.Start(ctx) }
func (l *LimitedConfidentialHTTPAction) Close() error                           { return l.inner.Close() }
func (l *LimitedConfidentialHTTPAction) HealthReport() map[string]error         { return l.inner.HealthReport() }
func (l *LimitedConfidentialHTTPAction) Name() string                           { return l.inner.Name() }
func (l *LimitedConfidentialHTTPAction) Description() string                    { return l.inner.Description() }
func (l *LimitedConfidentialHTTPAction) Ready() error                           { return l.inner.Ready() }
func (l *LimitedConfidentialHTTPAction) Initialise(ctx context.Context, deps core.StandardCapabilitiesDependencies) error {
	return l.inner.Initialise(ctx, deps)
}

// --- LimitedConsensusNoDAG ---

// LimitedConsensusNoDAG wraps a consensusserver.ConsensusCapability and enforces
// observation size limits from SimulationLimits.
type LimitedConsensusNoDAG struct {
	inner  consensusserver.ConsensusCapability
	limits *SimulationLimits
}

var _ consensusserver.ConsensusCapability = (*LimitedConsensusNoDAG)(nil)

func NewLimitedConsensusNoDAG(inner consensusserver.ConsensusCapability, limits *SimulationLimits) *LimitedConsensusNoDAG {
	return &LimitedConsensusNoDAG{inner: inner, limits: limits}
}

func (l *LimitedConsensusNoDAG) Simple(ctx context.Context, metadata commonCap.RequestMetadata, input *sdkpb.SimpleConsensusInputs) (*commonCap.ResponseAndMetadata[*valuespb.Value], caperrors.Error) {
	// Check observation size
	obsLimit := l.limits.ConsensusObservationSizeLimit()
	if obsLimit > 0 {
		inputSize := proto.Size(input)
		if inputSize > obsLimit {
			return nil, caperrors.NewPublicUserError(
				fmt.Errorf("simulation limit exceeded: consensus observation size %d bytes exceeds limit of %d bytes", inputSize, obsLimit),
				caperrors.ResourceExhausted,
			)
		}
	}

	return l.inner.Simple(ctx, metadata, input)
}

func (l *LimitedConsensusNoDAG) Report(ctx context.Context, metadata commonCap.RequestMetadata, input *sdkpb.ReportRequest) (*commonCap.ResponseAndMetadata[*sdkpb.ReportResponse], caperrors.Error) {
	// Report size is engine-enforced, delegate as-is
	return l.inner.Report(ctx, metadata, input)
}

func (l *LimitedConsensusNoDAG) Start(ctx context.Context) error        { return l.inner.Start(ctx) }
func (l *LimitedConsensusNoDAG) Close() error                           { return l.inner.Close() }
func (l *LimitedConsensusNoDAG) HealthReport() map[string]error         { return l.inner.HealthReport() }
func (l *LimitedConsensusNoDAG) Name() string                           { return l.inner.Name() }
func (l *LimitedConsensusNoDAG) Description() string                    { return l.inner.Description() }
func (l *LimitedConsensusNoDAG) Ready() error                           { return l.inner.Ready() }
func (l *LimitedConsensusNoDAG) Initialise(ctx context.Context, deps core.StandardCapabilitiesDependencies) error {
	return l.inner.Initialise(ctx, deps)
}

// --- LimitedEVMChain ---

// LimitedEVMChain wraps an evmserver.ClientCapability and enforces chain write
// report size and gas limits from SimulationLimits.
type LimitedEVMChain struct {
	inner  evmserver.ClientCapability
	limits *SimulationLimits
}

var _ evmserver.ClientCapability = (*LimitedEVMChain)(nil)

func NewLimitedEVMChain(inner evmserver.ClientCapability, limits *SimulationLimits) *LimitedEVMChain {
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

func (l *LimitedEVMChain) ChainSelector() uint64      { return l.inner.ChainSelector() }
func (l *LimitedEVMChain) Start(ctx context.Context) error { return l.inner.Start(ctx) }
func (l *LimitedEVMChain) Close() error                { return l.inner.Close() }
func (l *LimitedEVMChain) HealthReport() map[string]error { return l.inner.HealthReport() }
func (l *LimitedEVMChain) Name() string                { return l.inner.Name() }
func (l *LimitedEVMChain) Description() string         { return l.inner.Description() }
func (l *LimitedEVMChain) Ready() error                { return l.inner.Ready() }
func (l *LimitedEVMChain) Initialise(ctx context.Context, deps core.StandardCapabilitiesDependencies) error {
	return l.inner.Initialise(ctx, deps)
}

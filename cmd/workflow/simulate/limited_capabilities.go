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
	consensusserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/consensus/server"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
	sdkpb "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
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

func (l *LimitedHTTPAction) Start(ctx context.Context) error { return l.inner.Start(ctx) }
func (l *LimitedHTTPAction) Close() error                    { return l.inner.Close() }
func (l *LimitedHTTPAction) HealthReport() map[string]error  { return l.inner.HealthReport() }
func (l *LimitedHTTPAction) Name() string                    { return l.inner.Name() }
func (l *LimitedHTTPAction) Description() string             { return l.inner.Description() }
func (l *LimitedHTTPAction) Ready() error                    { return l.inner.Ready() }
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

func (l *LimitedConfidentialHTTPAction) Start(ctx context.Context) error { return l.inner.Start(ctx) }
func (l *LimitedConfidentialHTTPAction) Close() error                    { return l.inner.Close() }
func (l *LimitedConfidentialHTTPAction) HealthReport() map[string]error {
	return l.inner.HealthReport()
}
func (l *LimitedConfidentialHTTPAction) Name() string        { return l.inner.Name() }
func (l *LimitedConfidentialHTTPAction) Description() string { return l.inner.Description() }
func (l *LimitedConfidentialHTTPAction) Ready() error        { return l.inner.Ready() }
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

func (l *LimitedConsensusNoDAG) Start(ctx context.Context) error { return l.inner.Start(ctx) }
func (l *LimitedConsensusNoDAG) Close() error                    { return l.inner.Close() }
func (l *LimitedConsensusNoDAG) HealthReport() map[string]error  { return l.inner.HealthReport() }
func (l *LimitedConsensusNoDAG) Name() string                    { return l.inner.Name() }
func (l *LimitedConsensusNoDAG) Description() string             { return l.inner.Description() }
func (l *LimitedConsensusNoDAG) Ready() error                    { return l.inner.Ready() }
func (l *LimitedConsensusNoDAG) Initialise(ctx context.Context, deps core.StandardCapabilitiesDependencies) error {
	return l.inner.Initialise(ctx, deps)
}

package simulate

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/confidentialhttp"
	customhttp "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/http"
	"github.com/smartcontractkit/chainlink-common/pkg/config"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
	sdkpb "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
)

type capabilityBaseStub struct{}

func (capabilityBaseStub) Start(context.Context) error { return nil }
func (capabilityBaseStub) Close() error                { return nil }
func (capabilityBaseStub) HealthReport() map[string]error {
	return map[string]error{}
}
func (capabilityBaseStub) Name() string        { return "stub" }
func (capabilityBaseStub) Description() string { return "stub" }
func (capabilityBaseStub) Ready() error        { return nil }
func (capabilityBaseStub) Initialise(context.Context, core.StandardCapabilitiesDependencies) error {
	return nil
}

type httpClientCapabilityStub struct {
	capabilityBaseStub
	sendRequestFn    func(context.Context, commonCap.RequestMetadata, *customhttp.Request) (*commonCap.ResponseAndMetadata[*customhttp.Response], caperrors.Error)
	sendRequestCalls int
}

func (s *httpClientCapabilityStub) SendRequest(ctx context.Context, metadata commonCap.RequestMetadata, input *customhttp.Request) (*commonCap.ResponseAndMetadata[*customhttp.Response], caperrors.Error) {
	s.sendRequestCalls++
	if s.sendRequestFn != nil {
		return s.sendRequestFn(ctx, metadata, input)
	}
	return nil, nil
}

type confidentialHTTPClientCapabilityStub struct {
	capabilityBaseStub
	sendRequestFn    func(context.Context, commonCap.RequestMetadata, *confidentialhttp.ConfidentialHTTPRequest) (*commonCap.ResponseAndMetadata[*confidentialhttp.HTTPResponse], caperrors.Error)
	sendRequestCalls int
}

func (s *confidentialHTTPClientCapabilityStub) SendRequest(ctx context.Context, metadata commonCap.RequestMetadata, input *confidentialhttp.ConfidentialHTTPRequest) (*commonCap.ResponseAndMetadata[*confidentialhttp.HTTPResponse], caperrors.Error) {
	s.sendRequestCalls++
	if s.sendRequestFn != nil {
		return s.sendRequestFn(ctx, metadata, input)
	}
	return nil, nil
}

type consensusCapabilityStub struct {
	capabilityBaseStub
	simpleFn    func(context.Context, commonCap.RequestMetadata, *sdkpb.SimpleConsensusInputs) (*commonCap.ResponseAndMetadata[*valuespb.Value], caperrors.Error)
	reportFn    func(context.Context, commonCap.RequestMetadata, *sdkpb.ReportRequest) (*commonCap.ResponseAndMetadata[*sdkpb.ReportResponse], caperrors.Error)
	simpleCalls int
	reportCalls int
}

func (s *consensusCapabilityStub) Simple(ctx context.Context, metadata commonCap.RequestMetadata, input *sdkpb.SimpleConsensusInputs) (*commonCap.ResponseAndMetadata[*valuespb.Value], caperrors.Error) {
	s.simpleCalls++
	if s.simpleFn != nil {
		return s.simpleFn(ctx, metadata, input)
	}
	return nil, nil
}

func (s *consensusCapabilityStub) Report(ctx context.Context, metadata commonCap.RequestMetadata, input *sdkpb.ReportRequest) (*commonCap.ResponseAndMetadata[*sdkpb.ReportResponse], caperrors.Error) {
	s.reportCalls++
	if s.reportFn != nil {
		return s.reportFn(ctx, metadata, input)
	}
	return nil, nil
}

func newTestLimits(t *testing.T) *SimulationLimits {
	t.Helper()
	limits, err := DefaultLimits()
	require.NoError(t, err)
	return limits
}

func TestLimitedHTTPActionRejectsOversizedRequest(t *testing.T) {
	t.Parallel()

	limits := newTestLimits(t)
	limits.Workflows.HTTPAction.RequestSizeLimit.DefaultValue = 4

	inner := &httpClientCapabilityStub{}
	wrapper := NewLimitedHTTPAction(inner, limits)

	resp, err := wrapper.SendRequest(context.Background(), commonCap.RequestMetadata{}, &customhttp.Request{Body: []byte("12345")})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "HTTP request body size 5 bytes exceeds limit of 4 bytes")
	assert.Equal(t, 0, inner.sendRequestCalls)
}

func TestLimitedHTTPActionAppliesTimeoutAndAllowsBoundarySizedPayloads(t *testing.T) {
	t.Parallel()

	limits := newTestLimits(t)
	limits.Workflows.HTTPAction.RequestSizeLimit.DefaultValue = 4
	limits.Workflows.HTTPAction.ResponseSizeLimit.DefaultValue = 5
	limits.Workflows.HTTPAction.ConnectionTimeout.DefaultValue = 2 * time.Second

	inner := &httpClientCapabilityStub{
		sendRequestFn: func(ctx context.Context, _ commonCap.RequestMetadata, input *customhttp.Request) (*commonCap.ResponseAndMetadata[*customhttp.Response], caperrors.Error) {
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			remaining := time.Until(deadline)
			assert.LessOrEqual(t, remaining, 2*time.Second)
			assert.Greater(t, remaining, time.Second)
			assert.Equal(t, []byte("1234"), input.GetBody())
			return &commonCap.ResponseAndMetadata[*customhttp.Response]{
				Response: &customhttp.Response{Body: []byte("12345")},
			}, nil
		},
	}

	wrapper := NewLimitedHTTPAction(inner, limits)
	resp, err := wrapper.SendRequest(context.Background(), commonCap.RequestMetadata{}, &customhttp.Request{Body: []byte("1234")})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, []byte("12345"), resp.Response.GetBody())
	assert.Equal(t, 1, inner.sendRequestCalls)
}

func TestLimitedHTTPActionRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	limits := newTestLimits(t)
	limits.Workflows.HTTPAction.ResponseSizeLimit.DefaultValue = 3

	inner := &httpClientCapabilityStub{
		sendRequestFn: func(context.Context, commonCap.RequestMetadata, *customhttp.Request) (*commonCap.ResponseAndMetadata[*customhttp.Response], caperrors.Error) {
			return &commonCap.ResponseAndMetadata[*customhttp.Response]{
				Response: &customhttp.Response{Body: []byte("1234")},
			}, nil
		},
	}

	wrapper := NewLimitedHTTPAction(inner, limits)
	resp, err := wrapper.SendRequest(context.Background(), commonCap.RequestMetadata{}, &customhttp.Request{})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "HTTP response body size 4 bytes exceeds limit of 3 bytes")
	assert.Equal(t, 1, inner.sendRequestCalls)
}

func TestLimitedHTTPActionPassesThroughInnerError(t *testing.T) {
	t.Parallel()

	limits := newTestLimits(t)
	expectedResp := &commonCap.ResponseAndMetadata[*customhttp.Response]{Response: &customhttp.Response{Body: []byte("ok")}}
	expectedErr := caperrors.NewPublicUserError(errors.New("boom"), caperrors.ResourceExhausted)

	inner := &httpClientCapabilityStub{
		sendRequestFn: func(context.Context, commonCap.RequestMetadata, *customhttp.Request) (*commonCap.ResponseAndMetadata[*customhttp.Response], caperrors.Error) {
			return expectedResp, expectedErr
		},
	}

	wrapper := NewLimitedHTTPAction(inner, limits)
	resp, err := wrapper.SendRequest(context.Background(), commonCap.RequestMetadata{}, &customhttp.Request{})
	require.Error(t, err)
	assert.Same(t, expectedResp, resp)
	assert.True(t, expectedErr.Equals(err))
	assert.Equal(t, 1, inner.sendRequestCalls)
}

func TestLimitedConfidentialHTTPActionRejectsOversizedRequest(t *testing.T) {
	t.Parallel()

	limits := newTestLimits(t)
	limits.Workflows.ConfidentialHTTP.RequestSizeLimit.DefaultValue = 4

	inner := &confidentialHTTPClientCapabilityStub{}
	wrapper := NewLimitedConfidentialHTTPAction(inner, limits)

	resp, err := wrapper.SendRequest(context.Background(), commonCap.RequestMetadata{}, &confidentialhttp.ConfidentialHTTPRequest{
		Request: &confidentialhttp.HTTPRequest{Body: &confidentialhttp.HTTPRequest_BodyString{BodyString: "12345"}},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "confidential HTTP request body size 5 bytes exceeds limit of 4 bytes")
	assert.Equal(t, 0, inner.sendRequestCalls)
}

func TestLimitedConfidentialHTTPActionAppliesTimeoutAndAllowsBoundarySizedPayloads(t *testing.T) {
	t.Parallel()

	limits := newTestLimits(t)
	limits.Workflows.ConfidentialHTTP.RequestSizeLimit.DefaultValue = 4
	limits.Workflows.ConfidentialHTTP.ResponseSizeLimit.DefaultValue = 5
	limits.Workflows.ConfidentialHTTP.ConnectionTimeout.DefaultValue = 2 * time.Second

	inner := &confidentialHTTPClientCapabilityStub{
		sendRequestFn: func(ctx context.Context, _ commonCap.RequestMetadata, input *confidentialhttp.ConfidentialHTTPRequest) (*commonCap.ResponseAndMetadata[*confidentialhttp.HTTPResponse], caperrors.Error) {
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			remaining := time.Until(deadline)
			assert.LessOrEqual(t, remaining, 2*time.Second)
			assert.Greater(t, remaining, time.Second)
			assert.Equal(t, []byte("1234"), input.GetRequest().GetBodyBytes())
			return &commonCap.ResponseAndMetadata[*confidentialhttp.HTTPResponse]{
				Response: &confidentialhttp.HTTPResponse{Body: []byte("12345")},
			}, nil
		},
	}

	wrapper := NewLimitedConfidentialHTTPAction(inner, limits)
	resp, err := wrapper.SendRequest(context.Background(), commonCap.RequestMetadata{}, &confidentialhttp.ConfidentialHTTPRequest{
		Request: &confidentialhttp.HTTPRequest{Body: &confidentialhttp.HTTPRequest_BodyBytes{BodyBytes: []byte("1234")}},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, []byte("12345"), resp.Response.GetBody())
	assert.Equal(t, 1, inner.sendRequestCalls)
}

func TestLimitedConfidentialHTTPActionRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	limits := newTestLimits(t)
	limits.Workflows.ConfidentialHTTP.ResponseSizeLimit.DefaultValue = 3

	inner := &confidentialHTTPClientCapabilityStub{
		sendRequestFn: func(context.Context, commonCap.RequestMetadata, *confidentialhttp.ConfidentialHTTPRequest) (*commonCap.ResponseAndMetadata[*confidentialhttp.HTTPResponse], caperrors.Error) {
			return &commonCap.ResponseAndMetadata[*confidentialhttp.HTTPResponse]{
				Response: &confidentialhttp.HTTPResponse{Body: []byte("1234")},
			}, nil
		},
	}

	wrapper := NewLimitedConfidentialHTTPAction(inner, limits)
	resp, err := wrapper.SendRequest(context.Background(), commonCap.RequestMetadata{}, &confidentialhttp.ConfidentialHTTPRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "confidential HTTP response body size 4 bytes exceeds limit of 3 bytes")
	assert.Equal(t, 1, inner.sendRequestCalls)
}

func TestLimitedConsensusNoDAGSimpleRejectsOversizedObservation(t *testing.T) {
	t.Parallel()

	input := &sdkpb.SimpleConsensusInputs{
		Observation: &sdkpb.SimpleConsensusInputs_Error{Error: strings.Repeat("x", 64)},
	}

	limits := newTestLimits(t)
	limits.Workflows.Consensus.ObservationSizeLimit.DefaultValue = config.Size(proto.Size(input) - 1)

	inner := &consensusCapabilityStub{}
	wrapper := NewLimitedConsensusNoDAG(inner, limits)

	resp, err := wrapper.Simple(context.Background(), commonCap.RequestMetadata{}, input)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "consensus observation size")
	assert.Equal(t, 0, inner.simpleCalls)
}

func TestLimitedConsensusNoDAGSimpleDelegatesWhenWithinLimit(t *testing.T) {
	t.Parallel()

	input := &sdkpb.SimpleConsensusInputs{
		Observation: &sdkpb.SimpleConsensusInputs_Error{Error: "ok"},
	}

	limits := newTestLimits(t)
	limits.Workflows.Consensus.ObservationSizeLimit.DefaultValue = config.Size(proto.Size(input))
	expectedResp := &commonCap.ResponseAndMetadata[*valuespb.Value]{Response: &valuespb.Value{}}

	inner := &consensusCapabilityStub{
		simpleFn: func(_ context.Context, _ commonCap.RequestMetadata, got *sdkpb.SimpleConsensusInputs) (*commonCap.ResponseAndMetadata[*valuespb.Value], caperrors.Error) {
			assert.Same(t, input, got)
			return expectedResp, nil
		},
	}

	wrapper := NewLimitedConsensusNoDAG(inner, limits)
	resp, err := wrapper.Simple(context.Background(), commonCap.RequestMetadata{}, input)
	require.NoError(t, err)
	assert.Same(t, expectedResp, resp)
	assert.Equal(t, 1, inner.simpleCalls)
}

func TestLimitedConsensusNoDAGReportDelegates(t *testing.T) {
	t.Parallel()

	input := &sdkpb.ReportRequest{EncodedPayload: []byte("payload")}
	expectedResp := &commonCap.ResponseAndMetadata[*sdkpb.ReportResponse]{Response: &sdkpb.ReportResponse{RawReport: []byte("report")}}

	inner := &consensusCapabilityStub{
		reportFn: func(_ context.Context, _ commonCap.RequestMetadata, got *sdkpb.ReportRequest) (*commonCap.ResponseAndMetadata[*sdkpb.ReportResponse], caperrors.Error) {
			assert.Same(t, input, got)
			return expectedResp, nil
		},
	}

	wrapper := NewLimitedConsensusNoDAG(inner, newTestLimits(t))
	resp, err := wrapper.Report(context.Background(), commonCap.RequestMetadata{}, input)
	require.NoError(t, err)
	assert.Same(t, expectedResp, resp)
	assert.Equal(t, 1, inner.reportCalls)
}

package simulate

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	httptypedapi "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/http"
	httpserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/http/server"
	"github.com/smartcontractkit/chainlink-common/pkg/config"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
	"github.com/smartcontractkit/chainlink-common/pkg/workflows"
	"github.com/smartcontractkit/chainlink/v2/core/services/workflows/events"
)

var errHTTPTriggerRateLimited = fmt.Errorf("HTTP trigger rate limited")

var _ services.Service = (*ManualHTTPTriggerService)(nil)
var _ httpserver.HTTPCapability = (*ManualHTTPTriggerService)(nil)

const manualHTTPTriggerServiceName = "HttpTriggerService"
const manualHTTPTriggerID = "http-trigger@1.0.0-alpha"

var manualHTTPTriggerInfo = capabilities.MustNewCapabilityInfo(
	manualHTTPTriggerID,
	capabilities.CapabilityTypeTrigger,
	"A trigger that uses an HTTP request to run a workflow.",
)

type ManualHTTPTriggerService struct {
	capabilities.CapabilityInfo

	lggr logger.Logger

	mu          sync.RWMutex
	callbackCh  map[string]chan capabilities.TriggerAndId[*httptypedapi.Payload]
	workflowIDs map[string]string
	inputs      map[string]*httptypedapi.Config
	limiters    map[string]*rate.Limiter
	eventSeq    uint64
	port        int
	rateLimit   *config.Rate
}

func NewManualHTTPTriggerService(parentLggr logger.Logger, port int, rateLimit *config.Rate) *ManualHTTPTriggerService {
	return &ManualHTTPTriggerService{
		CapabilityInfo: manualHTTPTriggerInfo,
		lggr:           logger.Named(parentLggr, "HTTPTriggerService"),
		callbackCh:     make(map[string]chan capabilities.TriggerAndId[*httptypedapi.Payload]),
		workflowIDs:    make(map[string]string),
		inputs:         make(map[string]*httptypedapi.Config),
		limiters:       make(map[string]*rate.Limiter),
		port:           port,
		rateLimit:      rateLimit,
	}
}

func (f *ManualHTTPTriggerService) RegisterTrigger(ctx context.Context, triggerID string, metadata capabilities.RequestMetadata, input *httptypedapi.Config) (<-chan capabilities.TriggerAndId[*httptypedapi.Payload], caperrors.Error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.inputs[triggerID] = input
	f.callbackCh[triggerID] = make(chan capabilities.TriggerAndId[*httptypedapi.Payload], 1)
	f.workflowIDs[triggerID] = metadata.WorkflowID
	if f.rateLimit != nil && f.rateLimit.Limit > 0 && f.rateLimit.Burst > 0 {
		f.limiters[triggerID] = rate.NewLimiter(f.rateLimit.Limit, f.rateLimit.Burst)
	}
	return f.callbackCh[triggerID], nil
}

func (f *ManualHTTPTriggerService) UnregisterTrigger(ctx context.Context, triggerID string, metadata capabilities.RequestMetadata, input *httptypedapi.Config) caperrors.Error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.inputs, triggerID)
	delete(f.callbackCh, triggerID)
	delete(f.workflowIDs, triggerID)
	delete(f.limiters, triggerID)
	return nil
}

func (f *ManualHTTPTriggerService) AckEvent(ctx context.Context, triggerID string, eventID string, method string) caperrors.Error {
	return nil
}

func (f *ManualHTTPTriggerService) Initialise(ctx context.Context, dependencies core.StandardCapabilitiesDependencies) error {
	f.lggr.Debugf("Initialising %s", manualHTTPTriggerServiceName)
	return f.Start(ctx)
}

func (f *ManualHTTPTriggerService) ManualTrigger(ctx context.Context, triggerID string, payload *httptypedapi.Payload) error {
	f.mu.RLock()
	workflowID, workflowExists := f.workflowIDs[triggerID]
	input := f.inputs[triggerID]
	callbackCh := f.callbackCh[triggerID]
	limiter := f.limiters[triggerID]
	f.mu.RUnlock()

	if !workflowExists {
		f.lggr.Errorw("workflowID not found for triggerID", "triggerID", triggerID)
		workflowID = "unknownWorkflow"
	}
	if input == nil {
		f.lggr.Errorw("input is nil or not found for triggerID", "triggerID", triggerID)
		return fmt.Errorf("input not found for triggerID")
	}
	if callbackCh == nil {
		return fmt.Errorf("callback channel not found for triggerID")
	}
	if limiter != nil && !limiter.Allow() {
		return fmt.Errorf("%w: %s", errHTTPTriggerRateLimited, f.rateLimit.String())
	}

	if payload == nil {
		var err error
		payload, err = f.listenForTriggerPayload(ctx)
		if err != nil {
			return fmt.Errorf("gateway: %w", err)
		}
	}

	// triggerIndex defaults to zero in simulation
	var triggerIndex int
	triggerEvent := f.createManualTriggerEvent(payload)
	workflowExecutionID, err := workflows.GenerateExecutionIDWithTriggerIndex(workflowID, triggerEvent.Id, triggerIndex)
	if err != nil {
		f.lggr.Errorw("failed to generate execution ID", "err", err)
		workflowExecutionID = ""
	}
	if err := events.EmitTriggerExecutionStarted(ctx, map[string]string{}, triggerEvent.Id, workflowExecutionID); err != nil {
		f.lggr.Errorw("failed to emit trigger execution started event", "err", err)
	}

	select {
	case callbackCh <- triggerEvent:
		return nil
	case <-ctx.Done():
		f.lggr.Debug("ManualTrigger cancelled due to context cancellation")
		return ctx.Err()
	}
}

func (f *ManualHTTPTriggerService) listenForTriggerPayload(ctx context.Context) (*httptypedapi.Payload, error) {
	payloadCh, closeServer, err := startHTTPListenPayloadServer(ctx, f.port)
	if err != nil {
		return nil, err
	}
	defer closeServer()

	select {
	case payload := <-payloadCh:
		return payload, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *ManualHTTPTriggerService) createManualTriggerEvent(payload *httptypedapi.Payload) capabilities.TriggerAndId[*httptypedapi.Payload] {
	seq := atomic.AddUint64(&f.eventSeq, 1)
	return capabilities.TriggerAndId[*httptypedapi.Payload]{
		Trigger: payload,
		Id:      fmt.Sprintf("manual-http-trigger-%d-%d", time.Now().UnixNano(), seq),
	}
}

func (f *ManualHTTPTriggerService) Start(ctx context.Context) error {
	f.lggr.Debug("Starting HTTP Trigger Capability")
	return nil
}

func (f *ManualHTTPTriggerService) Close() error {
	f.lggr.Debug("Closing HTTP Trigger Capability")
	return nil
}

func (f *ManualHTTPTriggerService) HealthReport() map[string]error {
	return map[string]error{f.Name(): nil}
}

func (f *ManualHTTPTriggerService) Name() string {
	return f.lggr.Name()
}

func (f *ManualHTTPTriggerService) Description() string {
	return "Manual HTTP Trigger Service"
}

func (f *ManualHTTPTriggerService) Ready() error {
	return nil
}

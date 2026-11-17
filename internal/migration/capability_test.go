package migration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"

	cronpb "github.com/smartcontractkit/capabilities/scheduler/cron/protos"
	"github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	capreg "github.com/smartcontractkit/chainlink-common/pkg/capabilities/registry"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
)

const (
	recorderID     = "recorder@1.0.0"
	devConfig      = "capabilities.scheduler.cron.x.v1.Config"
	prodConfig     = "capabilities.scheduler.cron.v1.Config"
	devPayloadURL  = "type.googleapis.com/capabilities.scheduler.cron.x.v1.Payload"
	prodPayloadURL = "type.googleapis.com/capabilities.scheduler.cron.v1.Payload"
)

// recorder records the payload names it is asked with and replies with a development cron Payload.
type recorder struct {
	capabilities.ExecutableAndTriggerCapability
	got []string
}

func (r *recorder) Info(context.Context) (capabilities.CapabilityInfo, error) {
	return capabilities.NewCapabilityInfo(recorderID, capabilities.CapabilityTypeCombined, "records requests")
}

func (r *recorder) Execute(_ context.Context, request capabilities.CapabilityRequest) (capabilities.CapabilityResponse, error) {
	r.got = append(r.got, string(request.Payload.MessageName()))
	a, err := anypb.New(&cronpb.Payload{})
	return capabilities.CapabilityResponse{Payload: a}, err
}

func (r *recorder) RegisterTrigger(_ context.Context, request capabilities.TriggerRegistrationRequest) (<-chan capabilities.TriggerResponse, error) {
	r.got = append(r.got, string(request.Payload.MessageName()))
	a, err := anypb.New(&cronpb.Payload{})
	if err != nil {
		return nil, err
	}

	events := make(chan capabilities.TriggerResponse, 1)
	events <- capabilities.TriggerResponse{Event: capabilities.TriggerEvent{ID: "1", Payload: a}}
	close(events)
	return events, nil
}

func (r *recorder) UnregisterTrigger(_ context.Context, request capabilities.TriggerRegistrationRequest) error {
	r.got = append(r.got, string(request.Payload.MessageName()))
	return nil
}

// register adds a wrapped recorder to a registry, returning both.
func register(t *testing.T) (*recorder, *capreg.Registry) {
	t.Helper()
	r := &recorder{}
	registry := capreg.NewRegistry(logger.Test(t))
	require.NoError(t, registry.Add(t.Context(), Wrap(r)))
	return r, registry
}

func executeWith(t *testing.T, payload *anypb.Any) (*recorder, capabilities.CapabilityResponse) {
	t.Helper()
	r, registry := register(t)
	c, err := registry.GetExecutable(t.Context(), recorderID)
	require.NoError(t, err)

	response, err := c.Execute(t.Context(), capabilities.CapabilityRequest{Payload: payload})
	require.NoError(t, err)
	return r, response
}

func anyNamed(t *testing.T, name string) *anypb.Any {
	t.Helper()
	a, err := anypb.New(&cronpb.Config{Schedule: "* * * * *"})
	require.NoError(t, err)
	a.TypeUrl = "type.googleapis.com/" + name
	return a
}

func TestExecuteProduction(t *testing.T) {
	r, response := executeWith(t, anyNamed(t, prodConfig))

	assert.Equal(t, []string{devConfig}, r.got)
	assert.Equal(t, prodPayloadURL, response.Payload.TypeUrl)
}

func TestExecuteDevelopment(t *testing.T) {
	r, response := executeWith(t, anyNamed(t, devConfig))

	assert.Equal(t, []string{devConfig}, r.got)
	assert.Equal(t, devPayloadURL, response.Payload.TypeUrl)
}

func TestExecuteNotReleased(t *testing.T) {
	payload, err := anypb.New(&emptypb.Empty{})
	require.NoError(t, err)

	r, response := executeWith(t, payload)

	assert.Equal(t, []string{"google.protobuf.Empty"}, r.got)
	assert.Equal(t, devPayloadURL, response.Payload.TypeUrl)
}

func TestTriggerProduction(t *testing.T) {
	r, registry := register(t)
	c, err := registry.GetTrigger(t.Context(), recorderID)
	require.NoError(t, err)

	request := capabilities.TriggerRegistrationRequest{TriggerID: "t", Payload: anyNamed(t, prodConfig)}
	events, err := c.RegisterTrigger(t.Context(), request)
	require.NoError(t, err)

	// The registry keeps the events channel open until the trigger is unregistered, so read the one event sent.
	select {
	case event := <-events:
		require.NoError(t, event.Err)
		assert.Equal(t, prodPayloadURL, event.Event.Payload.TypeUrl)
	case <-time.After(10 * time.Second):
		t.Fatal("no trigger event")
	}

	require.NoError(t, c.UnregisterTrigger(t.Context(), request))
	assert.Equal(t, []string{devConfig, devConfig}, r.got)
}

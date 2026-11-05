package simulate

import (
	"context"
	"sync"

	"github.com/jonboulle/clockwork"
	"github.com/samber/do/v2"

	"github.com/smartcontractkit/capabilities/libs/capability"
	"github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	capreg "github.com/smartcontractkit/chainlink-common/pkg/capabilities/registry"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
)

// Capabilities builds the simulator's capabilities from the capabilities repo, all from one injector. What the
// simulator injects lets it trigger them early: skipping Clock fires their timers now.
type Capabilities struct {
	Clock *SkipClock
	// Services are what the capabilities started, for the caller to close, even when Add fails.
	Services []services.Service

	lggr     logger.Logger
	injector *capability.Injector
	// selected is the only trigger registration the capabilities are asked for, set before the engine registers.
	selected  string
	triggered chan struct{}
}

// initCapabilities clones the injector capabilities are embedded from, for the simulator to override what it
// needs to on it. Capabilities provide their own configs, so they run with their defaults unless overridden.
func initCapabilities(lggr logger.Logger) *Capabilities {
	return &Capabilities{
		Clock:     NewSkipClock(),
		lggr:      lggr,
		injector:  capability.EmbedInjector.Clone(),
		triggered: make(chan struct{}, 1),
	}
}

// Add builds and starts the capability ctor builds, then registers it.
func (c *Capabilities) Add[T capability.Capability](ctx context.Context, registry *capreg.Registry, ctor func(do.Injector) (T, error)) error {
	// StartCapabilities provides the real clock after cloning the injector, so only WithProvider overrides it.
	built, svcs, err := capability.StartCapabilities(ctx, c.lggr, c.injector, 1, ctor,
		capability.WithProvider(func(i *capability.Injector) error {
			return i.ProvideValue[clockwork.Clock](c.Clock)
		}))
	c.Services = append(c.Services, svcs...)
	if err != nil {
		return err
	}

	return registry.Add(ctx, &selectableTrigger{ExecutableAndTriggerCapability: built[0], capabilities: c})
}

// Select makes triggerID the only trigger registration passed on to the capabilities. Call it before the engine
// registers its triggers.
func (c *Capabilities) Select(triggerID string) {
	c.selected = triggerID
}

// WaitForTrigger waits for the selected trigger to send an event.
func (c *Capabilities) WaitForTrigger(ctx context.Context) error {
	select {
	case <-c.triggered:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// selectableTrigger passes on only the selected trigger registration. Any other is a no-op whose events
// channel closes when it is unregistered.
type selectableTrigger struct {
	capabilities.ExecutableAndTriggerCapability
	capabilities *Capabilities
	unselected   sync.Map // trigger ID to chan capabilities.TriggerResponse
}

func (s *selectableTrigger) RegisterTrigger(ctx context.Context, request capabilities.TriggerRegistrationRequest) (<-chan capabilities.TriggerResponse, error) {
	if request.TriggerID != s.capabilities.selected {
		events := make(chan capabilities.TriggerResponse)
		s.unselected.Store(request.TriggerID, events)
		return events, nil
	}

	events, err := s.ExecutableAndTriggerCapability.RegisterTrigger(ctx, request)
	if err != nil {
		return nil, err
	}

	forwarded := make(chan capabilities.TriggerResponse)
	go func() {
		defer close(forwarded)
		for event := range events {
			forwarded <- event
			select {
			case s.capabilities.triggered <- struct{}{}:
			default:
			}
		}
	}()

	return forwarded, nil
}

func (s *selectableTrigger) UnregisterTrigger(ctx context.Context, request capabilities.TriggerRegistrationRequest) error {
	if events, ok := s.unselected.LoadAndDelete(request.TriggerID); ok {
		close(events.(chan capabilities.TriggerResponse))
		return nil
	}

	return s.ExecutableAndTriggerCapability.UnregisterTrigger(ctx, request)
}

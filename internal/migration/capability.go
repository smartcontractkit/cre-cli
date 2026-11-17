package migration

import (
	"context"
	"slices"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/smartcontractkit/capabilities/libs/capability/feature"
	"github.com/smartcontractkit/chainlink-common/pkg/capabilities"
)

func Wrap(c capabilities.BaseCapability) capabilities.BaseCapability {
	switch c := c.(type) {
	case capabilities.ExecutableAndTriggerCapability:
		return &executableAndTrigger{ExecutableAndTriggerCapability: c}
	case capabilities.ExecutableCapability:
		return &executable{ExecutableCapability: c}
	case capabilities.TriggerCapability:
		return &trigger{TriggerCapability: c}
	default:
		return c
	}
}

type executableAndTrigger struct {
	capabilities.ExecutableAndTriggerCapability
}

func (c *executableAndTrigger) Execute(ctx context.Context, request capabilities.CapabilityRequest) (capabilities.CapabilityResponse, error) {
	return execute(ctx, c.ExecutableAndTriggerCapability, request)
}

func (c *executableAndTrigger) RegisterTrigger(ctx context.Context, request capabilities.TriggerRegistrationRequest) (<-chan capabilities.TriggerResponse, error) {
	return registerTrigger(ctx, c.ExecutableAndTriggerCapability, request)
}

func (c *executableAndTrigger) UnregisterTrigger(ctx context.Context, request capabilities.TriggerRegistrationRequest) error {
	return unregisterTrigger(ctx, c.ExecutableAndTriggerCapability, request)
}

type executable struct {
	capabilities.ExecutableCapability
}

func (c *executable) Execute(ctx context.Context, request capabilities.CapabilityRequest) (capabilities.CapabilityResponse, error) {
	return execute(ctx, c.ExecutableCapability, request)
}

type trigger struct {
	capabilities.TriggerCapability
}

func (c *trigger) RegisterTrigger(ctx context.Context, request capabilities.TriggerRegistrationRequest) (<-chan capabilities.TriggerResponse, error) {
	return registerTrigger(ctx, c.TriggerCapability, request)
}

func (c *trigger) UnregisterTrigger(ctx context.Context, request capabilities.TriggerRegistrationRequest) error {
	return unregisterTrigger(ctx, c.TriggerCapability, request)
}

func execute(ctx context.Context, c capabilities.ExecutableCapability, request capabilities.CapabilityRequest) (capabilities.CapabilityResponse, error) {
	var production bool
	request.Payload, production = toDevelopment(request.Payload)
	response, err := c.Execute(ctx, request)
	if err != nil || !production {
		return response, err
	}

	response.Payload, err = toProduction(response.Payload)
	return response, err
}

func registerTrigger(ctx context.Context, c capabilities.TriggerCapability, request capabilities.TriggerRegistrationRequest) (<-chan capabilities.TriggerResponse, error) {
	var production bool
	request.Payload, production = toDevelopment(request.Payload)
	events, err := c.RegisterTrigger(ctx, request)
	if err != nil || !production {
		return events, err
	}

	forwarded := make(chan capabilities.TriggerResponse)
	go func() {
		defer close(forwarded)
		for event := range events {
			if event.Err == nil {
				event.Event.Payload, event.Err = toProduction(event.Event.Payload)
			}

			forwarded <- event
		}
	}()

	return forwarded, nil
}

func unregisterTrigger(ctx context.Context, c capabilities.TriggerCapability, request capabilities.TriggerRegistrationRequest) error {
	request.Payload, _ = toDevelopment(request.Payload)
	return c.UnregisterTrigger(ctx, request)
}

// toDevelopment renames a production message to the development message it is released from, reporting whether
// it did. Any other message is returned as is, for the capability to read or reject.
func toDevelopment(a *anypb.Any) (*anypb.Any, bool) {
	if a == nil {
		return a, false
	}

	// The production message may be registered too, by the SDKs, so it being registered doesn't mean the
	// capability reads it: the development message is what it reads when there is one.
	name := a.MessageName()
	dev, ok := developmentName(name)
	if !ok {
		return a, false
	}

	return &anypb.Any{TypeUrl: strings.TrimSuffix(a.TypeUrl, string(name)) + string(dev), Value: a.Value}, true
}

// developmentName finds the registered development message prod is the production name of.
func developmentName(prod protoreflect.FullName) (protoreflect.FullName, bool) {
	parts := strings.Split(string(prod), ".")
	for i := 1; i < len(parts); i++ {
		candidate := protoreflect.FullName(strings.Join(slices.Insert(slices.Clone(parts), i, feature.Development), "."))
		mt, err := protoregistry.GlobalTypes.FindMessageByName(candidate)
		if err != nil {
			continue
		}

		if name, err := feature.ProductionName(mt.Descriptor()); err == nil && name == prod {
			return candidate, true
		}
	}

	return "", false
}

// toProduction renames a development message to its production name.
func toProduction(a *anypb.Any) (*anypb.Any, error) {
	if a == nil {
		return a, nil
	}

	name := a.MessageName()
	mt, err := protoregistry.GlobalTypes.FindMessageByName(name)
	if err != nil {
		return nil, err
	}

	prod, err := feature.ProductionName(mt.Descriptor())
	if err != nil {
		return nil, err
	}

	return &anypb.Any{TypeUrl: strings.TrimSuffix(a.TypeUrl, string(name)) + string(prod), Value: a.Value}, nil
}

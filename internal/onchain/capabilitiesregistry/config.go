package capabilitiesregistry

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	capabilitiespb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"
)

// ParseCapabilityConfiguration decodes raw capability config bytes from the registry contract.
func ParseCapabilityConfiguration(raw []byte) (capabilities.CapabilityConfiguration, error) {
	cconf := &capabilitiespb.CapabilityConfig{}
	if err := proto.Unmarshal(raw, cconf); err != nil {
		return capabilities.CapabilityConfiguration{}, fmt.Errorf("unmarshal capability config: %w", err)
	}

	dc, err := values.FromMapValueProto(cconf.DefaultConfig)
	if err != nil {
		return capabilities.CapabilityConfiguration{}, fmt.Errorf("decode default config: %w", err)
	}

	return capabilities.CapabilityConfiguration{DefaultConfig: dc}, nil
}

package capabilitiesregistry

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	capabilitiespb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"
)

// VaultCapabilityRegistryConfig is the on-chain defaultConfig shape for vault@1.0.0.
type VaultCapabilityRegistryConfig struct {
	VaultPublicKey string `mapstructure:"VaultPublicKey"`
	Threshold      int    `mapstructure:"Threshold"`
}

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

// ParseVaultCapabilityConfig extracts VaultPublicKey and Threshold from vault@1.0.0 config bytes.
func ParseVaultCapabilityConfig(raw []byte) (*VaultCapabilityRegistryConfig, error) {
	cfg, err := ParseCapabilityConfiguration(raw)
	if err != nil {
		return nil, err
	}

	out := &VaultCapabilityRegistryConfig{}
	if err := cfg.DefaultConfig.UnwrapTo(out); err != nil {
		return nil, fmt.Errorf("unwrap vault capability config: %w", err)
	}
	if out.VaultPublicKey == "" {
		return nil, fmt.Errorf("VaultPublicKey is not provided in the capability config")
	}
	if out.Threshold <= 0 {
		return nil, fmt.Errorf("invalid Threshold in the capability config")
	}
	return out, nil
}

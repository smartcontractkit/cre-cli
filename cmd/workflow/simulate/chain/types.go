package chain

import (
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"
)

// ChainClient is an opaque handle to a chain-specific RPC client.
// Each family casts this to its concrete type internally.
type ChainClient interface{}

// ChainConfig identifies a supported chain within a family.
type ChainConfig struct {
	Selector  uint64
	Forwarder string // family-specific forwarding address
}

// CapabilityConfig holds everything a family needs to register capabilities.
type CapabilityConfig struct {
	Registry   *capabilities.Registry
	Clients    map[uint64]ChainClient
	Forwarders map[uint64]string
	PrivateKey interface{} // family-specific key type; EVM uses *ecdsa.PrivateKey
	Broadcast  bool
	Limits     interface{} // *SimulationLimits from parent package, or nil
	Logger     logger.Logger
}

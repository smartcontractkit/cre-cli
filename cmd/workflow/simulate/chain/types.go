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

// Limits exposes the chain-write limit accessors a family needs at
// capability-registration time. Implementations live in the parent simulate
// package; this interface is defined here so CapabilityConfig stays in the
// chain package without an import cycle.
type Limits interface {
	ChainWriteReportSizeLimit() int
	ChainWriteEVMGasLimit() uint64
}

// CapabilityConfig holds everything a family needs to register capabilities.
type CapabilityConfig struct {
	Registry   *capabilities.Registry
	Clients    map[uint64]ChainClient
	Forwarders map[uint64]string
	PrivateKey interface{} // family-specific key type; EVM uses *ecdsa.PrivateKey
	Broadcast  bool
	Limits     Limits // nil disables limit enforcement
	Logger     logger.Logger
}

// TriggerParams carries family-agnostic inputs needed to resolve trigger data
// for a given chain trigger. FamilyInputs is a free-form bag of CLI-supplied
// strings; each family interprets the keys it knows about and ignores the rest.
type TriggerParams struct {
	Clients      map[uint64]ChainClient
	Interactive  bool
	FamilyInputs map[string]string
}

package chain

import (
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"
)

// ChainClient is an opaque handle to a chain-specific RPC client.
// Each chain type casts this to its concrete type internally.
type ChainClient interface{}

// ChainConfig identifies a supported chain within a chain type.
type ChainConfig struct {
	Selector  uint64
	Forwarder string // chain-type-specific forwarding address
}

// ResolvedChains is the result of ChainType.ResolveClients: the RPC clients,
// forwarders, and any chain-type-agnostic metadata later interface methods
// (e.g. RunHealthCheck) depend on.
type ResolvedChains struct {
	Clients    map[uint64]ChainClient
	Forwarders map[uint64]string
	// ExperimentalSelectors marks selectors that came from experimental-chain
	// config rather than the chain type's built-in supported list. Used for
	// error labelling (e.g. "experimental chain N" vs a chain name).
	ExperimentalSelectors map[uint64]bool
}

// Limits is the common per-family limits contract enforced by the
// LimitedChain wrappers.
type Limits struct {
	ReportSize int
	GasLimit   uint64
}

// CapabilityConfig holds everything a chain type needs to register capabilities.
type CapabilityConfig struct {
	Registry   *capabilities.Registry
	Clients    map[uint64]ChainClient
	Forwarders map[uint64]string
	PrivateKey interface{} // chain-type-specific key type; EVM uses *ecdsa.PrivateKey
	Broadcast  bool
	Limits     *cresettings.Workflows // nil disables enforcement
	Logger     logger.Logger
}

// TriggerParams carries chain-type-agnostic inputs needed to resolve trigger data
// for a given chain trigger. ChainTypeInputs is a free-form bag of CLI-supplied
// strings; each chain type interprets the keys it knows about and ignores the rest.
type TriggerParams struct {
	Clients         map[uint64]ChainClient
	Interactive     bool
	ChainTypeInputs map[string]string
}

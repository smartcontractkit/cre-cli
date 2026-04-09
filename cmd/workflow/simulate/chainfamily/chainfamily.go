// Package chainfamily defines the adapter interface for multi-chain simulation support.
// Each chain family (EVM, Aptos, Solana, etc.) implements the Adapter interface and
// self-registers via init(). The core simulate flow iterates registered adapters
// instead of hardcoding chain-specific logic.
package chainfamily

import (
	"context"
	"io"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"
)

// ChainSelector is a uint64 chain selector from the chain-selectors package.
type ChainSelector = uint64

// SupportedChain describes a chain known to the simulator with its default config.
type SupportedChain struct {
	Selector  ChainSelector
	Forwarder string // hex address, family-specific format
}

// ExperimentalChainConfig holds per-chain overrides from experimental config,
// already filtered to a specific adapter's family by the core.
type ExperimentalChainConfig struct {
	ChainSelector ChainSelector
	RPCURL        string
	Forwarder     string
}

// SetupConfig contains everything an adapter needs to set itself up.
// The core populates this once; no viper dependency leaks into adapters.
// Capability registration happens separately via ChainRuntime.RegisterCapabilities.
type SetupConfig struct {
	Logger logger.Logger

	// RPCURLs maps chain name -> RPC URL, scoped to this adapter's family.
	RPCURLs map[string]string

	// ExperimentalChains filtered to THIS adapter's family by the core.
	ExperimentalChains []ExperimentalChainConfig

	// DryRun is true when chain writes should be simulated (default behavior).
	DryRun bool

	// SecretsPath is the path to the secrets file, if any.
	SecretsPath string

	// FlagValues provides access to family-specific flag values registered via AddFlags.
	FlagValues func(name string) string

	// ChainWriteReportSizeLimit is the max report size in bytes (0 = no limit).
	ChainWriteReportSizeLimit int

	// ChainWriteGasLimit is the max gas limit for chain writes (0 = no limit).
	ChainWriteGasLimit uint64
}

// TriggerRequest contains everything an adapter needs to build a trigger function.
type TriggerRequest struct {
	TriggerID             string // e.g. "evm:ChainSelector:123@1.0.0 LogTrigger"
	TriggerRegistrationID string
	ChainSelector         ChainSelector
	PromptUser            bool // true = interactive mode
	FlagValues            func(name string) string
}

// ChainRuntime is the live handle returned by Setup(). The adapter owns its
// internal state (clients, fake chains) behind this interface. The core code
// never inspects chain internals - it just calls BuildTriggerFunc and Close.
type ChainRuntime interface {
	// RegisterCapabilities registers chain-specific capabilities with the
	// capability registry. Called during simulator initialization when the
	// registry is available, not during Setup().
	RegisterCapabilities(ctx context.Context, registry *capabilities.Registry) error

	// BuildTriggerFunc creates a function that fires the given chain trigger.
	BuildTriggerFunc(ctx context.Context, req TriggerRequest) (func() error, error)

	// OwnedSelectors returns all chain selectors this runtime manages.
	// Used for trigger routing without relying on GetSelectorFamily.
	OwnedSelectors() []ChainSelector

	// Services returns all services managed by this runtime for lifecycle management.
	Services() []services.Service

	// Close releases all resources (RPC clients, fake chains, etc.)
	io.Closer
}

// Adapter is the contract each chain family implements. Adapters self-register
// via init() + Register().
type Adapter interface {
	// Family returns the chain family identifier (e.g. "evm", "aptos", "solana").
	Family() string

	// SupportedChains returns default chain configs for this family.
	SupportedChains() []SupportedChain

	// AddFlags registers family-specific CLI flags on the simulate command.
	AddFlags(cmd *cobra.Command)

	// Setup dials RPCs, health-checks, and returns a ChainRuntime.
	// Returns (nil, nil) if no chains of this family are configured.
	Setup(ctx context.Context, cfg SetupConfig) (ChainRuntime, error)
}

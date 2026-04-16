package chain

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/chainlink-common/pkg/services"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// Factory constructs a ChainType with the logger the simulator uses.
// Registered at init() time; invoked during Build() at command runtime.
type Factory func(lggr *zerolog.Logger) ChainType

// ChainType defines what a chain type plugin must implement
// to participate in workflow simulation.
type ChainType interface {
	// Name returns the chain type identifier (e.g., "evm", "aptos").
	Name() string

	// ResolveClients creates RPC clients for all chains this chain type can
	// simulate, including both supported and experimental chains. Returns a
	// ResolvedChains bundle containing clients keyed by chain selector,
	// forwarder addresses, and any chain-type-agnostic metadata (e.g.
	// experimental-selector set) that later interface methods need.
	ResolveClients(v *viper.Viper) (ResolvedChains, error)

	// ResolveKey parses and validates this chain type's signing key from
	// settings. If broadcast is true, missing or default-sentinel keys
	// are a hard error; otherwise a sentinel may be used with a warning.
	// Returns the parsed key (chain-type-specific) or nil if the chain
	// type does not use a signing key.
	ResolveKey(creSettings *settings.Settings, broadcast bool) (interface{}, error)

	// ResolveTriggerData produces the chain-type-specific trigger payload for
	// a given chain selector, using runtime parameters from the caller.
	ResolveTriggerData(ctx context.Context, selector uint64, params TriggerParams) (interface{}, error)

	// RegisterCapabilities creates capability servers for this chain type's
	// chains and adds them to the registry. Returns the underlying services
	// (e.g., per-selector chain fakes) so the caller can manage their lifecycle.
	RegisterCapabilities(ctx context.Context, cfg CapabilityConfig) ([]services.Service, error)

	// ExecuteTrigger fires a chain-specific trigger for a given selector.
	// Each chain type defines what triggerData looks like.
	ExecuteTrigger(ctx context.Context, selector uint64, registrationID string, triggerData interface{}) error

	// HasSelector reports whether the chain type has a fully initialised
	// capability for the given selector after RegisterCapabilities ran.
	// Used by the trigger-setup loop to fail fast before a TriggerFunc is
	// assigned for a selector the chain type cannot actually dispatch against.
	HasSelector(selector uint64) bool

	// ParseTriggerChainSelector extracts a chain selector from a
	// trigger subscription ID string (e.g., "evm:ChainSelector:123@1.0.0").
	// Returns 0, false if the trigger doesn't belong to this chain type.
	ParseTriggerChainSelector(triggerID string) (uint64, bool)

	// RunHealthCheck validates RPC connectivity for all resolved clients.
	// The resolved argument is the same bundle ResolveClients returned,
	// threaded back by the caller so RunHealthCheck is self-contained and
	// does not depend on hidden state on the ChainType instance.
	RunHealthCheck(resolved ResolvedChains) error

	// SupportedChains returns the list of chains this chain type supports
	// out of the box (for display/documentation purposes).
	SupportedChains() []ChainConfig

	// CollectCLIInputs reads this chain type's CLI flags from viper and
	// returns them as key-value pairs for TriggerParams.ChainTypeInputs.
	CollectCLIInputs(v *viper.Viper) map[string]string
}

// CLIFlagDef describes a CLI flag a chain type needs registered.
type CLIFlagDef struct {
	Name         string
	Description  string
	DefaultValue string // empty string for string flags, or special handling
	FlagType     CLIFlagType
}

// CLIFlagType indicates the Go type of a CLI flag.
type CLIFlagType int

const (
	CLIFlagString CLIFlagType = iota
	CLIFlagInt
)

// registration bundles a factory with its CLI flag definitions.
type registration struct {
	factory  Factory
	flagDefs []CLIFlagDef
}

var (
	mu            sync.RWMutex
	registrations = make(map[string]registration)
	chainTypes    = make(map[string]ChainType)
)

// Register adds a chain type factory and its CLI flag definitions to the
// registry. Called from chain type package init(); the factory is invoked later
// in Build(). Panics on duplicate registration (programming error).
func Register(name string, factory Factory, flagDefs []CLIFlagDef) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := registrations[name]; exists {
		panic(fmt.Sprintf("chain type %q already registered", name))
	}
	registrations[name] = registration{factory: factory, flagDefs: flagDefs}
}

// Build instantiates every registered chain type with the given logger.
// Must be called once at command startup before All()/Get() return
// meaningful results.
func Build(lggr *zerolog.Logger) {
	mu.Lock()
	defer mu.Unlock()
	for name, reg := range registrations {
		chainTypes[name] = reg.factory(lggr)
	}
}

// Get returns a registered chain type by name.
func Get(name string) (ChainType, error) {
	mu.RLock()
	defer mu.RUnlock()
	ct, ok := chainTypes[name]
	if !ok {
		return nil, fmt.Errorf("unknown chain type %q; registered: %v", name, namesLocked())
	}
	return ct, nil
}

// All returns a copy of all registered chain types.
func All() map[string]ChainType {
	mu.RLock()
	defer mu.RUnlock()
	result := make(map[string]ChainType, len(chainTypes))
	for k, v := range chainTypes {
		result[k] = v
	}
	return result
}

// RegisterAllCLIFlags registers CLI flags from every registered chain type's
// flag definitions. Called at command setup time before Build().
func RegisterAllCLIFlags(cmd *cobra.Command) {
	mu.RLock()
	defer mu.RUnlock()
	for _, reg := range registrations {
		for _, def := range reg.flagDefs {
			switch def.FlagType {
			case CLIFlagInt:
				defaultVal := -1
				if def.DefaultValue != "" {
					if v, err := strconv.Atoi(def.DefaultValue); err == nil {
						defaultVal = v
					}
				}
				cmd.Flags().Int(def.Name, defaultVal, def.Description)
			default:
				cmd.Flags().String(def.Name, def.DefaultValue, def.Description)
			}
		}
	}
}

// CollectAllCLIInputs gathers CLI inputs from every registered chain type.
func CollectAllCLIInputs(v *viper.Viper) map[string]string {
	result := map[string]string{}
	for _, ct := range All() {
		for k, val := range ct.CollectCLIInputs(v) {
			result[k] = val
		}
	}
	return result
}

// namesLocked returns sorted chain type names. Caller must hold mu.
func namesLocked() []string {
	names := make([]string, 0, len(chainTypes))
	for k := range chainTypes {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Names returns sorted registered chain type names.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	return namesLocked()
}

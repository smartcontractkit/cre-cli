package chain

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/chainlink-common/pkg/services"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// Factory constructs a ChainFamily with the logger the simulator uses.
// Registered at init() time; invoked during Build() at command runtime.
type Factory func(lggr *zerolog.Logger) ChainFamily

// ChainFamily defines what a chain family plugin must implement
// to participate in workflow simulation.
type ChainFamily interface {
	// Name returns the family identifier (e.g., "evm", "aptos").
	Name() string

	// ResolveClients creates RPC clients for all chains this family
	// can simulate, including both supported and experimental chains.
	// Returns clients keyed by chain selector, and forwarder addresses
	// for chains that have them.
	ResolveClients(v *viper.Viper) (clients map[uint64]ChainClient, forwarders map[uint64]string, err error)

	// ResolveKey parses and validates this family's signing key from
	// settings. If broadcast is true, missing or default-sentinel keys
	// are a hard error; otherwise a sentinel may be used with a warning.
	// Returns the parsed key (family-specific type) or nil if the family
	// does not use a signing key.
	ResolveKey(creSettings *settings.Settings, broadcast bool) (interface{}, error)

	// ResolveTriggerData produces the family-specific trigger payload for
	// a given chain selector, using runtime parameters from the caller.
	ResolveTriggerData(ctx context.Context, selector uint64, params TriggerParams) (interface{}, error)

	// RegisterCapabilities creates capability servers for this family's
	// chains and adds them to the registry. Returns the underlying services
	// (e.g., per-selector chain fakes) so the caller can manage their lifecycle.
	RegisterCapabilities(ctx context.Context, cfg CapabilityConfig) ([]services.Service, error)

	// ExecuteTrigger fires a chain-specific trigger for a given selector.
	// Each family defines what triggerData looks like.
	ExecuteTrigger(ctx context.Context, selector uint64, registrationID string, triggerData interface{}) error

	// HasSelector reports whether the family has a fully initialised
	// capability for the given selector after RegisterCapabilities ran.
	// Used by the trigger-setup loop to fail fast before a TriggerFunc is
	// assigned for a selector the family cannot actually dispatch against.
	HasSelector(selector uint64) bool

	// ParseTriggerChainSelector extracts a chain selector from a
	// trigger subscription ID string (e.g., "evm:ChainSelector:123@1.0.0").
	// Returns 0, false if the trigger doesn't belong to this family.
	ParseTriggerChainSelector(triggerID string) (uint64, bool)

	// RunHealthCheck validates RPC connectivity for all resolved clients.
	RunHealthCheck(clients map[uint64]ChainClient) error

	// SupportedChains returns the list of chains this family supports
	// out of the box (for display/documentation purposes).
	SupportedChains() []ChainConfig
}

var (
	mu        sync.RWMutex
	factories = make(map[string]Factory)
	families  = make(map[string]ChainFamily)
)

// Register adds a chain family factory to the registry.
// Called from family package init(); the factory is invoked later in Build().
// Panics on duplicate registration (programming error).
func Register(name string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := factories[name]; exists {
		panic(fmt.Sprintf("chain family %q already registered", name))
	}
	factories[name] = factory
}

// Build instantiates every registered family with the given logger.
// Must be called once at command startup before All()/Get() return
// meaningful results.
func Build(lggr *zerolog.Logger) {
	mu.Lock()
	defer mu.Unlock()
	for name, factory := range factories {
		families[name] = factory(lggr)
	}
}

// Get returns a registered chain family by name.
func Get(name string) (ChainFamily, error) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := families[name]
	if !ok {
		return nil, fmt.Errorf("unknown chain family %q; registered: %v", name, Names())
	}
	return f, nil
}

// All returns a copy of all registered families.
func All() map[string]ChainFamily {
	mu.RLock()
	defer mu.RUnlock()
	result := make(map[string]ChainFamily, len(families))
	for k, v := range families {
		result[k] = v
	}
	return result
}

// Names returns sorted registered family names.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(families))
	for k := range families {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

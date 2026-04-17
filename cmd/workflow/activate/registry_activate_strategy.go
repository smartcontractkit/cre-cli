package activate

import (
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// registryActivateStrategy encapsulates target-specific activate logic.
type registryActivateStrategy interface {
	Activate() error
}

// newRegistryActivateStrategy returns the appropriate strategy for the given target.
func newRegistryActivateStrategy(resolvedRegistry settings.ResolvedRegistry, h *handler) (registryActivateStrategy, error) {
	if resolvedRegistry.Type() == settings.RegistryTypeOffChain {
		return newPrivateRegistryActivateStrategy(h), nil
	}
	return newOnchainRegistryActivateStrategy(h)
}

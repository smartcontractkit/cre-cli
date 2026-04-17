package delete

import (
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// registryDeleteStrategy encapsulates target-specific delete logic.
type registryDeleteStrategy interface {
	Delete() error
}

// newRegistryDeleteStrategy returns the appropriate strategy for the given target.
func newRegistryDeleteStrategy(resolvedRegistry settings.ResolvedRegistry, h *handler) (registryDeleteStrategy, error) {
	if resolvedRegistry.Type() == settings.RegistryTypeOffChain {
		return newPrivateRegistryDeleteStrategy(h), nil
	}
	return newOnchainRegistryDeleteStrategy(h)
}

package pause

import (
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// registryPauseStrategy encapsulates target-specific pause logic.
type registryPauseStrategy interface {
	Pause() error
}

// newRegistryPauseStrategy returns the appropriate strategy for the given target.
func newRegistryPauseStrategy(resolvedRegistry settings.ResolvedRegistry, h *handler) (registryPauseStrategy, error) {
	if resolvedRegistry.Type() == settings.RegistryTypeOffChain {
		return newPrivateRegistryPauseStrategy(h), nil
	}
	return newOnchainRegistryPauseStrategy(h)
}

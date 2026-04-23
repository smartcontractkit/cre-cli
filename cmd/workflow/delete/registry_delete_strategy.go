package delete

import (
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

type WorkflowToDelete struct {
	ID        string
	Owner     string
	DonFamily string
	Tag       string
	BinaryURL string
	Status    string
	RawID     any // Holds the registry-specific ID type ([32]byte for on-chain, string for private)
}

// registryDeleteStrategy encapsulates target-specific delete logic.
type registryDeleteStrategy interface {
	FetchWorkflows() ([]WorkflowToDelete, error)
	DeleteWorkflows(workflows []WorkflowToDelete) error
}

// newRegistryDeleteStrategy returns the appropriate strategy for the given target.
func newRegistryDeleteStrategy(resolvedRegistry settings.ResolvedRegistry, h *handler) (registryDeleteStrategy, error) {
	if resolvedRegistry.Type() == settings.RegistryTypeOffChain {
		return newPrivateRegistryDeleteStrategy(h), nil
	}
	return newOnchainRegistryDeleteStrategy(h)
}

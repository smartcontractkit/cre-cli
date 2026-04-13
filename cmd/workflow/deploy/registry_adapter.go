package deploy

import (
	"errors"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

// errDeployHalted is a sentinel returned by RunPreDeployChecks when the deploy
// must stop without error (e.g. MSIG auto-link requires a manual step before
// re-running the command).
var errDeployHalted = errors.New("deploy halted")

// registryAdapter encapsulates target-specific deployment logic.
// The orchestrator calls these methods in a fixed sequence with common steps
// (artifact upload) between RunPreDeployChecks and Upsert.
type registryAdapter interface {
	// RunPreDeployChecks validates readiness and runs registry-specific
	// prechecks (ownership linking, duplicate detection, etc.).
	// Return errDeployHalted to stop the deploy without returning an error.
	RunPreDeployChecks() error

	// Upsert registers or updates the workflow in the target registry
	// and displays the result.
	Upsert() error
}

// resolveTargetRegistry resolves the deploy target details and returns the
// adapter that handles target-specific registry operations.
func resolveTargetRegistry(previewPrivateRegistry bool, envSet *environments.EnvironmentSet, factory client.Factory, h *handler) (registryTarget, registryAdapter, error) {
	target, err := resolveRegistryTarget(previewPrivateRegistry, envSet, factory)
	if err != nil {
		return registryTarget{}, nil, err
	}

	return target, newRegistryAdapter(target, h), nil
}

// newRegistryAdapter returns the appropriate adapter for the given target.
func newRegistryAdapter(target registryTarget, h *handler) registryAdapter {
	if target.isPrivate() {
		return newPrivateRegistryAdapter(h)
	}
	return newOnchainRegistryAdapter(h)
}

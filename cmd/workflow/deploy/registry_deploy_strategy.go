package deploy

import (
	"errors"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// errDeployHalted is a sentinel returned by RunPreDeployChecks when the deploy
// must stop without error (e.g. MSIG auto-link requires a manual step before
// re-running the command).
var errDeployHalted = errors.New("deploy halted")

// registryDeployStrategy encapsulates target-specific deployment logic.
// The orchestrator calls these methods in a fixed sequence with common steps
// (artifact upload) between RunPreDeployChecks and Upsert.
type registryDeployStrategy interface {
	// RunPreDeployChecks validates readiness and runs registry-specific
	// prechecks (ownership linking, duplicate detection, etc.).
	// Return errDeployHalted to stop the deploy without returning an error.
	RunPreDeployChecks() error

	// CheckWorkflowExists returns whether a same-name workflow exists for this
	// registry target and includes the existing workflow status for updates.
	CheckWorkflowExists(workflowOwner, workflowName, workflowTag, workflowID string) (bool, *uint8, error)

	// Upsert registers or updates the workflow in the target registry
	// and displays the result.
	Upsert() error
}

// newRegistryDeployStrategy returns the appropriate strategy for the given target.
func newRegistryDeployStrategy(resolvedRegistry settings.ResolvedRegistry, h *handler) (registryDeployStrategy, error) {
	if resolvedRegistry.Type() == settings.RegistryTypeOffChain {
		return newPrivateRegistryDeployStrategy(h), nil
	}
	return newOnchainRegistryDeployStrategy(h)
}

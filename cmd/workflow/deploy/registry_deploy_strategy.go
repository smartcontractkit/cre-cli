package deploy

import (
	"context"
	"errors"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// errDeployHalted is a sentinel returned by RunPreDeployChecks when the deploy
// must stop without error (e.g. MSIG auto-link requires a manual step before
// re-running the command).
var errDeployHalted = errors.New("deploy halted")

// errWorkflowUnchanged is a sentinel returned by CheckWorkflowExists when a
// registered workflow has the same ID as the artifact being deployed.
var errWorkflowUnchanged = errors.New("workflow unchanged")

// registryDeployStrategy encapsulates target-specific deployment logic.
// The orchestrator calls these methods in a fixed sequence with common steps
// (artifact upload) between RunPreDeployChecks and Upsert.
type registryDeployStrategy interface {
	// RunPreDeployChecks validates readiness and runs registry-specific
	// prechecks (ownership linking, duplicate detection, etc.).
	// Return errDeployHalted to stop the deploy without returning an error.
	RunPreDeployChecks(ctx context.Context) error

	// CheckWorkflowExists returns whether a same-name workflow exists for this
	// registry target and includes the existing workflow status for updates.
	// When the existing workflow ID matches workflowID, exists is true and
	// errWorkflowUnchanged is returned to block redeployment of identical artifacts.
	CheckWorkflowExists(ctx context.Context, workflowOwner, workflowName, workflowTag, workflowID string) (bool, *uint8, error)

	// Upsert registers or updates the workflow in the target registry
	// and displays the result.
	Upsert(ctx context.Context) error
}

// newRegistryDeployStrategy returns the appropriate strategy for the given target.
func newRegistryDeployStrategy(ctx context.Context, resolvedRegistry settings.ResolvedRegistry, h *handler) (registryDeployStrategy, error) {
	if resolvedRegistry.Type() == settings.RegistryTypeOffChain {
		return newPrivateRegistryDeployStrategy(h), nil
	}
	return newOnchainRegistryDeployStrategy(ctx, h)
}

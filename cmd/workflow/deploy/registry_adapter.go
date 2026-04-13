package deploy

import "errors"

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

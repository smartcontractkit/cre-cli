package deploy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/environments"
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

type registryTargetType string

const (
	registryTargetOnchain registryTargetType = "onchain"
	registryTargetPrivate registryTargetType = "private"
)

type registryTarget struct {
	targetType registryTargetType
}

func (t registryTarget) isPrivate() bool {
	return t.targetType == registryTargetPrivate
}

// resolveRegistryTarget determines the target workflow registry from inputs and
// environment. Preview-specific naming (--preview-private-registry, STAGING
// gate) is confined to this function; the returned target is preview-agnostic.
func resolveRegistryTarget(previewPrivateRegistry bool, envSet *environments.EnvironmentSet) (registryTarget, error) {
	if previewPrivateRegistry {
		if err := validatePrivateRegistryAllowed(envSet); err != nil {
			return registryTarget{}, err
		}
		return registryTarget{targetType: registryTargetPrivate}, nil
	}

	return registryTarget{targetType: registryTargetOnchain}, nil
}

// validatePrivateRegistryAllowed enforces the STAGING-only preview gate.
func validatePrivateRegistryAllowed(envSet *environments.EnvironmentSet) error {
	displayEnvName := envSet.EnvName
	if displayEnvName == "" {
		displayEnvName = environments.DefaultEnv
	}

	if !strings.EqualFold(displayEnvName, "STAGING") {
		return fmt.Errorf("--preview-private-registry is only available in the STAGING environment (current: %s)", displayEnvName)
	}
	return nil
}

// newRegistryDeployStrategy returns the appropriate strategy for the given target.
func newRegistryDeployStrategy(targetWorkflowRegistry registryTarget, h *handler) registryDeployStrategy {
	if targetWorkflowRegistry.isPrivate() {
		return newPrivateRegistryDeployStrategy(h)
	}
	return newOnchainRegistryDeployStrategy(h)
}

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
	if !strings.EqualFold(envSet.EnvName, "STAGING") {
		return fmt.Errorf("--preview-private-registry is only available in the STAGING environment (current: %s)", envSet.EnvName)
	}
	return nil
}

// resolveTargetRegistry resolves the deploy target details and returns the
// adapter that handles target-specific registry operations.
func resolveTargetRegistry(previewPrivateRegistry bool, envSet *environments.EnvironmentSet, h *handler) (registryTarget, registryAdapter, error) {
	target, err := resolveRegistryTarget(previewPrivateRegistry, envSet)
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

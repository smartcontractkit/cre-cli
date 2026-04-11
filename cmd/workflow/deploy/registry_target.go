package deploy

import (
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

type registryTargetType string

const (
	registryTargetOnchain registryTargetType = "onchain"
	registryTargetPrivate registryTargetType = "private"
)

type registryTarget struct {
	targetType registryTargetType

	// Onchain-specific fields, populated when targetType == registryTargetOnchain.
	onchainChainSelector uint64
	onchainAddress       string
}

// resolveRegistryTarget determines the target workflow registry from inputs and
// environment. Preview-specific naming (--preview-private-registry, STAGING
// gate) is confined to this function; the returned target is preview-agnostic.
func resolveRegistryTarget(previewPrivateRegistry bool, envSet *environments.EnvironmentSet, factory client.Factory) (registryTarget, error) {
	if previewPrivateRegistry {
		if err := validatePrivateRegistryAllowed(envSet); err != nil {
			return registryTarget{}, err
		}
		warnIfTxFlagsIgnored(factory)
		return registryTarget{targetType: registryTargetPrivate}, nil
	}

	chainSelector, err := settings.GetChainSelectorByChainName(envSet.WorkflowRegistryChainName)
	if err != nil {
		return registryTarget{}, fmt.Errorf("failed to get chain selector for chain %q: %w", envSet.WorkflowRegistryChainName, err)
	}

	return registryTarget{
		targetType:           registryTargetOnchain,
		onchainChainSelector: chainSelector,
		onchainAddress:       envSet.WorkflowRegistryAddress,
	}, nil
}

func (t registryTarget) isPrivate() bool {
	return t.targetType == registryTargetPrivate
}

// validatePrivateRegistryAllowed enforces the STAGING-only preview gate.
func validatePrivateRegistryAllowed(envSet *environments.EnvironmentSet) error {
	if !strings.EqualFold(envSet.EnvName, "STAGING") {
		return fmt.Errorf("--preview-private-registry is only available in the STAGING environment (current: %s)", envSet.EnvName)
	}
	return nil
}

func warnIfTxFlagsIgnored(factory client.Factory) {
	if factory.GetTxType() != client.Regular {
		ui.Warning("Transaction type flags (--unsigned, --changeset, etc.) are ignored when deploying to the private registry")
	}
}

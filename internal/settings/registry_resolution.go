package settings

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

// ResolvedRegistry holds the fully resolved registry configuration for a
// workflow command. It is built from either the static EnvironmentSet defaults
// or from a tenant context registry entry selected via deployment-registry.
type ResolvedRegistry struct {
	ID          string
	Type        string // "on-chain" or "off-chain"
	Address     string
	ChainName   string
	DonFamily   string
	ExplorerURL string
}

// ResolveRegistry maps an optional deployment-registry value to a concrete
// ResolvedRegistry. When deploymentRegistry is empty the static EnvironmentSet
// values are used (backwards-compatible default). When set, it is looked up in
// tenantCtx.Registries and the matching entry is used. Off-chain registries
// are rejected in production environments.
func ResolveRegistry(
	deploymentRegistry string,
	tenantCtx *tenantctx.EnvironmentContext,
	envSet *environments.EnvironmentSet,
) (*ResolvedRegistry, error) {
	if deploymentRegistry == "" {
		return defaultFromEnvironmentSet(envSet), nil
	}

	if tenantCtx == nil {
		return nil, fmt.Errorf("deployment-registry %q is set but user context is not available — run `cre login` and retry", deploymentRegistry)
	}

	reg := findRegistry(tenantCtx.Registries, deploymentRegistry)
	if reg == nil {
		return nil, fmt.Errorf("registry %q not found in context.yaml; available: [%s]",
			deploymentRegistry, availableIDs(tenantCtx.Registries))
	}

	if reg.Type == "off-chain" {
		if isProduction(envSet) {
			return nil, fmt.Errorf("off-chain (private) registries are not yet supported in production")
		}
		return &ResolvedRegistry{
			ID:        reg.ID,
			Type:      reg.Type,
			DonFamily: tenantCtx.DefaultDonFamily,
		}, nil
	}

	resolved := &ResolvedRegistry{
		ID:          reg.ID,
		Type:        reg.Type,
		DonFamily:   tenantCtx.DefaultDonFamily,
		ExplorerURL: envSet.WorkflowRegistryChainExplorerURL,
	}

	if reg.Address != nil {
		resolved.Address = *reg.Address
	}

	if reg.ChainSelector != nil {
		sel, err := strconv.ParseUint(*reg.ChainSelector, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid chain_selector %q for registry %q: %w", *reg.ChainSelector, reg.ID, err)
		}
		name, err := GetChainNameByChainSelector(sel)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve chain name for selector %d (registry %q): %w", sel, reg.ID, err)
		}
		resolved.ChainName = name
	}

	return resolved, nil
}

// TODO: remove this once off-chain routing is implemented
// RequireOnChainRegistry returns an error if the resolved registry is not
// on-chain. Use this in commands that only support on-chain workflow registries
// (deploy, pause, activate, delete) until off-chain routing is implemented.
func (r *ResolvedRegistry) RequireOnChainRegistry(commandName string) error {
	if r.Type != "on-chain" {
		return fmt.Errorf(
			"%s currently only supports on-chain registries; deployment-registry %q is %s",
			commandName, r.ID, r.Type,
		)
	}
	return nil
}

func defaultFromEnvironmentSet(envSet *environments.EnvironmentSet) *ResolvedRegistry {
	return &ResolvedRegistry{
		Type:        "on-chain",
		Address:     envSet.WorkflowRegistryAddress,
		ChainName:   envSet.WorkflowRegistryChainName,
		DonFamily:   envSet.DonFamily,
		ExplorerURL: envSet.WorkflowRegistryChainExplorerURL,
	}
}

func findRegistry(registries []*tenantctx.Registry, id string) *tenantctx.Registry {
	for _, r := range registries {
		if r.ID == id {
			return r
		}
	}
	return nil
}

func availableIDs(registries []*tenantctx.Registry) string {
	ids := make([]string, 0, len(registries))
	for _, r := range registries {
		ids = append(ids, r.ID)
	}
	return strings.Join(ids, ", ")
}

func isProduction(envSet *environments.EnvironmentSet) bool {
	return envSet.EnvName == "" || envSet.EnvName == environments.DefaultEnv
}

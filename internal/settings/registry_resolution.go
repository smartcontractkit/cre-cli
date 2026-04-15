package settings

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

// RegistryType distinguishes between on-chain and off-chain workflow registries.
type RegistryType string

const (
	RegistryTypeOnChain  RegistryType = "on-chain"
	RegistryTypeOffChain RegistryType = "off-chain"
)

// ResolvedRegistry is the interface implemented by both OnChainRegistry and
// OffChainRegistry. Commands type-switch on the concrete type to access
// type-specific fields, which prevents accidental use of fields that don't
// exist for a given registry kind.
type ResolvedRegistry interface {
	GetID() string
	GetType() RegistryType
	GetDonFamily() string
}

// OnChainRegistry holds the resolved configuration for an on-chain workflow
// registry. Address and ChainName are guaranteed non-empty.
type OnChainRegistry struct {
	ID          string
	Address     string
	ChainName   string
	DonFamily   string
	ExplorerURL string
}

func (r *OnChainRegistry) GetID() string         { return r.ID }
func (r *OnChainRegistry) GetType() RegistryType { return RegistryTypeOnChain }
func (r *OnChainRegistry) GetDonFamily() string  { return r.DonFamily }

// OffChainRegistry holds the resolved configuration for an off-chain (private)
// workflow registry. It has no on-chain address or chain.
type OffChainRegistry struct {
	ID        string
	DonFamily string
}

func (r *OffChainRegistry) GetID() string         { return r.ID }
func (r *OffChainRegistry) GetType() RegistryType { return RegistryTypeOffChain }
func (r *OffChainRegistry) GetDonFamily() string  { return r.DonFamily }

// ResolveRegistry maps an optional deployment-registry value to a concrete
// ResolvedRegistry. When deploymentRegistry is empty the static EnvironmentSet
// values are used (backwards-compatible default). When set, it is looked up in
// tenantCtx.Registries. On-chain entries must have a non-empty address.
// Off-chain registries are rejected in production environments.
func ResolveRegistry(
	deploymentRegistry string,
	tenantCtx *tenantctx.EnvironmentContext,
	envSet *environments.EnvironmentSet,
) (ResolvedRegistry, error) {
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

	if ParseRegistryType(reg.Type) == RegistryTypeOffChain {
		if isProduction(envSet) {
			return nil, fmt.Errorf("off-chain (private) registries are not yet supported in production")
		}
		return &OffChainRegistry{
			ID:        reg.ID,
			DonFamily: tenantCtx.DefaultDonFamily,
		}, nil
	}

	address := ""
	if reg.Address != nil {
		address = *reg.Address
	}
	if address == "" {
		return nil, fmt.Errorf("on-chain registry %q has no address in context.yaml", reg.ID)
	}

	resolved := &OnChainRegistry{
		ID:          reg.ID,
		Address:     address,
		DonFamily:   tenantCtx.DefaultDonFamily,
		ExplorerURL: envSet.WorkflowRegistryChainExplorerURL,
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

	if resolved.ChainName == "" {
		return nil, fmt.Errorf("on-chain registry %q has no chain_selector in context.yaml", reg.ID)
	}

	return resolved, nil
}

// ParseRegistryType converts a raw type string from context.yaml to a
// RegistryType. Unknown values default to on-chain.
func ParseRegistryType(raw string) RegistryType {
	if strings.EqualFold(raw, string(RegistryTypeOffChain)) || strings.EqualFold(raw, "off_chain") {
		return RegistryTypeOffChain
	}
	return RegistryTypeOnChain
}

func defaultFromEnvironmentSet(envSet *environments.EnvironmentSet) *OnChainRegistry {
	return &OnChainRegistry{
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

// AsOnChain asserts that r is an *OnChainRegistry. If it is not, it returns a
// descriptive error mentioning the command that required on-chain support.
func AsOnChain(r ResolvedRegistry, commandName string) (*OnChainRegistry, error) {
	if oc, ok := r.(*OnChainRegistry); ok {
		return oc, nil
	}
	return nil, fmt.Errorf(
		"%s currently only supports on-chain registries; deployment-registry %q is %s",
		commandName, r.GetID(), r.GetType(),
	)
}

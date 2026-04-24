package settings

import (
	"fmt"
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
	ID() string
	Type() RegistryType
	DonFamily() string
}

// OnChainRegistry holds the resolved configuration for an on-chain workflow
// registry. Address and ChainName are guaranteed non-empty.
type OnChainRegistry struct {
	id          string
	address     string
	chainName   string
	donFamily   string
	explorerURL string
}

func NewOnChainRegistry(id, address, chainName, donFamily, explorerURL string) *OnChainRegistry {
	return &OnChainRegistry{
		id:          id,
		address:     address,
		chainName:   chainName,
		donFamily:   donFamily,
		explorerURL: explorerURL,
	}
}

func (r *OnChainRegistry) ID() string          { return r.id }
func (r *OnChainRegistry) Type() RegistryType  { return RegistryTypeOnChain }
func (r *OnChainRegistry) DonFamily() string   { return r.donFamily }
func (r *OnChainRegistry) Address() string     { return r.address }
func (r *OnChainRegistry) ChainName() string   { return r.chainName }
func (r *OnChainRegistry) ExplorerURL() string { return r.explorerURL }

// OffChainRegistry holds the resolved configuration for an off-chain (private)
// workflow registry. It has no on-chain address or chain.
type OffChainRegistry struct {
	id        string
	donFamily string
}

func NewOffChainRegistry(id, donFamily string) *OffChainRegistry {
	return &OffChainRegistry{id: id, donFamily: donFamily}
}

func (r *OffChainRegistry) ID() string         { return r.id }
func (r *OffChainRegistry) Type() RegistryType { return RegistryTypeOffChain }
func (r *OffChainRegistry) DonFamily() string  { return r.donFamily }

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
		return nil, fmt.Errorf("registry %q not found in user context; available: [%s]",
			deploymentRegistry, availableIDs(tenantCtx.Registries))
	}

	if ParseRegistryType(reg.Type) == RegistryTypeOffChain {
		if isProduction(envSet) {
			return nil, fmt.Errorf("off-chain (private) registries are not yet supported in production")
		}
		return NewOffChainRegistry(reg.ID, tenantCtx.DefaultDonFamily), nil
	}

	if reg.Address == nil || *reg.Address == "" {
		return nil, fmt.Errorf("on-chain registry %q has no address in user context", reg.ID)
	}

	if reg.ChainSelector == nil {
		return nil, fmt.Errorf("on-chain registry %q has no chain_selector in user context", reg.ID)
	}
	chainName, err := ChainNameFromSelectorString(*reg.ChainSelector)
	if err != nil {
		return nil, fmt.Errorf("registry %q: %w", reg.ID, err)
	}

	return NewOnChainRegistry(
		reg.ID,
		*reg.Address,
		chainName,
		tenantCtx.DefaultDonFamily,
		envSet.WorkflowRegistryChainExplorerURL,
	), nil
}

// ParseRegistryType converts a raw type string from user context to a
// RegistryType. Unknown values default to on-chain.
func ParseRegistryType(raw string) RegistryType {
	if strings.EqualFold(raw, string(RegistryTypeOffChain)) || strings.EqualFold(raw, "off_chain") {
		return RegistryTypeOffChain
	}
	return RegistryTypeOnChain
}

func defaultFromEnvironmentSet(envSet *environments.EnvironmentSet) *OnChainRegistry {
	return NewOnChainRegistry(
		"",
		envSet.WorkflowRegistryAddress,
		envSet.WorkflowRegistryChainName,
		envSet.DonFamily,
		envSet.WorkflowRegistryChainExplorerURL,
	)
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
		commandName, r.ID(), r.Type(),
	)
}

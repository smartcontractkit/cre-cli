package settings

import (
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/registrytype"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

// RegistryType distinguishes between on-chain and off-chain workflow registries.
type RegistryType = registrytype.Type

const (
	RegistryTypeOnChain  = registrytype.OnChain
	RegistryTypeOffChain = registrytype.OffChain
	RegistryTypeUnknown  = registrytype.Unknown
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

// EffectiveDonFamily prefers envSet.DonFamily (CRE_CLI_DON_FAMILY at load); otherwise tenantCtx.DefaultDonFamily.
func EffectiveDonFamily(envSet *environments.EnvironmentSet, tenantCtx *tenantctx.EnvironmentContext) string {
	if envSet != nil {
		if v := strings.TrimSpace(envSet.DonFamily); v != "" {
			return v
		}
	}
	if tenantCtx != nil {
		return strings.TrimSpace(tenantCtx.DefaultDonFamily)
	}
	return ""
}

// ResolveRegistry maps an optional deployment-registry value to a concrete
// ResolvedRegistry. When deploymentRegistry is empty the static EnvironmentSet
// values are used (backwards-compatible default). When set, it is looked up in
// tenantCtx.Registries. On-chain entries must have a non-empty address.
func ResolveRegistry(
	deploymentRegistry string,
	tenantCtx *tenantctx.EnvironmentContext,
	envSet *environments.EnvironmentSet,
) (ResolvedRegistry, error) {
	if deploymentRegistry == "" {
		return defaultFromEnvironmentSet(envSet, tenantCtx), nil
	}

	if tenantCtx == nil {
		return nil, fmt.Errorf("deployment-registry %q is set but user context is not available — run `cre login` and retry", deploymentRegistry)
	}

	reg := findRegistry(tenantCtx.Registries, deploymentRegistry)
	if reg == nil {
		return nil, fmt.Errorf("registry %q not found in user context; available: [%s]",
			deploymentRegistry, availableIDs(tenantCtx.Registries))
	}

	regType, err := ParseRegistryType(reg.Type)
	if err != nil {
		return nil, fmt.Errorf("registry %q: %w", reg.ID, err)
	}

	switch regType {
	case RegistryTypeOffChain:
		return NewOffChainRegistry(reg.ID, EffectiveDonFamily(envSet, tenantCtx)), nil
	case RegistryTypeOnChain:
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
			EffectiveDonFamily(envSet, tenantCtx),
			envSet.WorkflowRegistryChainExplorerURL,
		), nil
	case RegistryTypeUnknown:
		return nil, fmt.Errorf(
			"registry %q is not supported by this CLI version (unrecognised type from server); run `cre login` after upgrading or choose a different deployment-registry",
			reg.ID,
		)
	default:
		return nil, fmt.Errorf("registry %q: %w", reg.ID, fmt.Errorf("unrecognised registry type %q", regType))
	}
}

// ParseRegistryType converts a raw type string from user context to a RegistryType.
func ParseRegistryType(raw string) (RegistryType, error) {
	return registrytype.Parse(raw)
}

func defaultFromEnvironmentSet(envSet *environments.EnvironmentSet, tenantCtx *tenantctx.EnvironmentContext) *OnChainRegistry {
	return NewOnChainRegistry(
		fmt.Sprintf("onchain:%s", envSet.WorkflowRegistryChainName),
		envSet.WorkflowRegistryAddress,
		envSet.WorkflowRegistryChainName,
		EffectiveDonFamily(envSet, tenantCtx),
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
		regType, err := ParseRegistryType(r.Type)
		if err != nil || regType == RegistryTypeUnknown {
			continue
		}
		ids = append(ids, r.ID)
	}
	return strings.Join(ids, ", ")
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

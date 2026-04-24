// Package workflowrender contains helpers for matching platform workflow
// rows to registries in the tenant context and rendering them as a table.
// It is shared by the workflow list and get commands.
//
// The list API returns workflowSource as either the raw registry id (e.g.
// "private"), a "contract:<chainSelector>:<0x…>" tuple for on-chain rows, or
// a "grpc:<…>" string for off-chain rows — so direct equality with the
// context registry id only works in the first case.
package workflowrender

import (
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

// Workflow is a type alias so callers can refer to the row type without
// importing the data client directly.
type Workflow = workflowdataclient.Workflow

// FilterRowsByRegistry returns only the rows that resolve to the given
// registry in the provided tenant context. A nil registry is treated as
// "no filter" and rows is returned unchanged.
func FilterRowsByRegistry(rows []Workflow, reg *tenantctx.Registry, all []*tenantctx.Registry) []Workflow {
	if reg == nil {
		return rows
	}
	out := make([]Workflow, 0, len(rows))
	for _, r := range rows {
		if workflowSourceMatchesRegistry(r.WorkflowSource, reg, all) {
			out = append(out, r)
		}
	}
	return out
}

// FindRegistry returns the registry entry with the matching ID, or nil.
func FindRegistry(registries []*tenantctx.Registry, id string) *tenantctx.Registry {
	for _, r := range registries {
		if r != nil && r.ID == id {
			return r
		}
	}
	return nil
}

// AvailableRegistryIDs returns a comma-separated list of registry IDs for
// use in error messages.
func AvailableRegistryIDs(registries []*tenantctx.Registry) string {
	ids := make([]string, 0, len(registries))
	for _, r := range registries {
		if r != nil {
			ids = append(ids, r.ID)
		}
	}
	return strings.Join(ids, ", ")
}

// ResolveWorkflowRegistry returns the registry in the tenant context that
// best matches the given workflowSource, or nil if none match.
func ResolveWorkflowRegistry(workflowSource string, registries []*tenantctx.Registry) *tenantctx.Registry {
	byID := registryByWorkflowSource(registries)
	if reg, ok := byID[workflowSource]; ok {
		return reg
	}

	if cr := findContractRegistry(workflowSource, registries); cr != nil {
		return cr
	}
	const contractPrefix = "contract:"
	if strings.HasPrefix(workflowSource, contractPrefix) {
		return nil
	}

	if strings.HasPrefix(workflowSource, "grpc:") {
		return resolveGrpcSourceRegistry(workflowSource, registries)
	}

	return nil
}

// RegistryIDOrSource returns the matched registry's ID, falling back to the
// raw workflowSource when no registry resolves cleanly.
func RegistryIDOrSource(workflowSource string, matched *tenantctx.Registry) string {
	if matched != nil {
		return matched.ID
	}
	return workflowSource
}

// RegistryEligibleForContractRows reports whether a registry can legitimately
// own on-chain ("contract:…") workflow sources.
func RegistryEligibleForContractRows(reg *tenantctx.Registry) bool {
	if reg == nil || !hasContractAddress(reg) {
		return false
	}
	if registryTypeOffChain(reg) {
		return false
	}
	return true
}

// ParseContractWorkflowSource splits a "contract:<chainSelector>:<addr>"
// workflow source. ok is false when the prefix is not present.
func ParseContractWorkflowSource(workflowSource string) (selector, addr string, ok bool) {
	const contractPrefix = "contract:"
	if !strings.HasPrefix(workflowSource, contractPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(workflowSource, contractPrefix)
	return strings.Cut(rest, ":")
}

func workflowSourceMatchesRegistry(workflowSource string, reg *tenantctx.Registry, all []*tenantctx.Registry) bool {
	if workflowSource == reg.ID {
		return true
	}

	const contractPrefix = "contract:"
	if strings.HasPrefix(workflowSource, contractPrefix) {
		return contractSourceMatchesRegistry(workflowSource, reg, all)
	}

	if strings.HasPrefix(workflowSource, "grpc:") {
		if !registryEligibleForGrpcRows(reg) {
			return false
		}
		resolved := resolveGrpcSourceRegistry(workflowSource, all)
		return resolved != nil && resolved.ID == reg.ID
	}

	return false
}

func registryTypeOffChain(reg *tenantctx.Registry) bool {
	if reg == nil {
		return false
	}
	t := strings.TrimSpace(strings.ReplaceAll(strings.ToLower(reg.Type), "_", "-"))
	return t == "off-chain" || strings.EqualFold(strings.TrimSpace(reg.Type), "OFF_CHAIN")
}

func hasContractAddress(reg *tenantctx.Registry) bool {
	return reg != nil && reg.Address != nil && strings.TrimSpace(*reg.Address) != ""
}

func registryEligibleForGrpcRows(reg *tenantctx.Registry) bool {
	if reg == nil {
		return false
	}
	if registryTypeOffChain(reg) {
		return true
	}
	if hasContractAddress(reg) {
		return false
	}
	return true
}

func contractSourceMatchesRegistry(workflowSource string, reg *tenantctx.Registry, all []*tenantctx.Registry) bool {
	found := findContractRegistry(workflowSource, all)
	return found != nil && found.ID == reg.ID
}

func findContractRegistry(workflowSource string, registries []*tenantctx.Registry) *tenantctx.Registry {
	const contractPrefix = "contract:"
	if !strings.HasPrefix(workflowSource, contractPrefix) {
		return nil
	}
	rest := strings.TrimPrefix(workflowSource, contractPrefix)
	selector, addr, ok := strings.Cut(rest, ":")
	if !ok || addr == "" {
		return nil
	}
	for _, r := range registries {
		if !RegistryEligibleForContractRows(r) {
			continue
		}
		if !addressesEqual(addr, *r.Address) {
			continue
		}
		if r.ChainSelector != nil && strings.TrimSpace(*r.ChainSelector) != "" &&
			strings.TrimSpace(*r.ChainSelector) != strings.TrimSpace(selector) {
			continue
		}
		return r
	}
	return nil
}

func resolveGrpcSourceRegistry(workflowSource string, all []*tenantctx.Registry) *tenantctx.Registry {
	if !strings.HasPrefix(workflowSource, "grpc:") {
		return nil
	}
	eligible := make([]*tenantctx.Registry, 0, len(all))
	for _, r := range all {
		if r != nil && registryEligibleForGrpcRows(r) {
			eligible = append(eligible, r)
		}
	}
	if len(eligible) == 1 {
		return eligible[0]
	}
	var match *tenantctx.Registry
	for _, r := range eligible {
		id := strings.ToLower(strings.TrimSpace(r.ID))
		if len(id) < 3 {
			continue
		}
		if strings.Contains(strings.ToLower(workflowSource), id) {
			if match != nil {
				return nil
			}
			match = r
		}
	}
	return match
}

func addressesEqual(a, b string) bool {
	return strings.EqualFold(
		strings.TrimPrefix(strings.TrimSpace(a), "0x"),
		strings.TrimPrefix(strings.TrimSpace(b), "0x"),
	)
}

func registryByWorkflowSource(registries []*tenantctx.Registry) map[string]*tenantctx.Registry {
	m := make(map[string]*tenantctx.Registry)
	for _, r := range registries {
		if r != nil {
			m[r.ID] = r
		}
	}
	return m
}

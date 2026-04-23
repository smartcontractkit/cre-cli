package list

import (
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

// Registry matching: user context stores registry id
// plus chain_selector and address, while the list API returns workflowSource as
// contract:<chainSelector>:<0x…> or grpc:… — not the manifest id string. Direct equality with
// reg.ID therefore only applies when the API echoes the same id (e.g. "private").

func filterRowsByRegistry(rows []Workflow, reg *tenantctx.Registry, all []*tenantctx.Registry) []Workflow {
	if reg == nil {
		return rows
	}
	out := make([]Workflow, 0, len(rows))
	for _, r := range rows {
		if rowMatchesRegistry(r.WorkflowSource, reg, all) {
			out = append(out, r)
		}
	}
	return out
}

func rowMatchesRegistry(workflowSource string, reg *tenantctx.Registry, all []*tenantctx.Registry) bool {
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

func registryEligibleForContractRows(reg *tenantctx.Registry) bool {
	if reg == nil || !hasContractAddress(reg) {
		return false
	}
	if registryTypeOffChain(reg) {
		return false
	}
	return true
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
		if !registryEligibleForContractRows(r) {
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

func parseContractWorkflowSource(workflowSource string) (selector, addr string, ok bool) {
	const contractPrefix = "contract:"
	if !strings.HasPrefix(workflowSource, contractPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(workflowSource, contractPrefix)
	return strings.Cut(rest, ":")
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

func resolveWorkflowRegistry(workflowSource string, registries []*tenantctx.Registry) *tenantctx.Registry {
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

func formatRegistryIDFromResolved(workflowSource string, matched *tenantctx.Registry) string {
	if matched != nil {
		return matched.ID
	}
	return workflowSource
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

func findRegistry(registries []*tenantctx.Registry, id string) *tenantctx.Registry {
	for _, r := range registries {
		if r != nil && r.ID == id {
			return r
		}
	}
	return nil
}

func availableRegistryIDs(registries []*tenantctx.Registry) string {
	ids := make([]string, 0, len(registries))
	for _, r := range registries {
		if r != nil {
			ids = append(ids, r.ID)
		}
	}
	return strings.Join(ids, ", ")
}

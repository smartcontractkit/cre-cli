package limits

import (
	"context"
	"fmt"
	"strings"

	"github.com/machinebox/graphql"

	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// GraphQLExecutor is the subset of the GraphQL client the command needs, so tests can inject a
// fake without a server.
type GraphQLExecutor interface {
	Execute(ctx context.Context, req *graphql.Request, resp any) error
}

const queryRegistryQuotas = `
query RegistryQuotas {
  registryQuotas {
    privateRegistry {
      workflows { donFamily active limit }
    }
    onchainRegistries {
      chainSelector
      address
      linkedOwners { linked pending limit }
      owners { owner workflows { donFamily active limit } }
    }
  }
}`

type quotaWorkflow struct {
	DonFamily string `json:"donFamily"`
	Active    int    `json:"active"`
	Limit     int    `json:"limit"`
}

type linkedOwnersQuota struct {
	Linked  []string `json:"linked"`
	Pending []string `json:"pending"`
	Limit   int      `json:"limit"`
}

type ownerWorkflowQuota struct {
	Owner     string          `json:"owner"`
	Workflows []quotaWorkflow `json:"workflows"`
}

type privateRegistryQuota struct {
	Workflows []quotaWorkflow `json:"workflows"`
}

type onchainRegistryQuota struct {
	ChainSelector string               `json:"chainSelector"`
	Address       string               `json:"address"`
	LinkedOwners  linkedOwnersQuota    `json:"linkedOwners"`
	Owners        []ownerWorkflowQuota `json:"owners"`
}

type registryQuotas struct {
	PrivateRegistry   *privateRegistryQuota  `json:"privateRegistry"`
	OnchainRegistries []onchainRegistryQuota `json:"onchainRegistries"`
}

type registryQuotasResponse struct {
	RegistryQuotas registryQuotas `json:"registryQuotas"`
}

// loadQuotas fetches the quota for the organization. machinebox/graphql decodes data before
// returning the first GraphQL error, so a partial result (for example owners: null after a chain
// failure) is still usable. Only a response with no quotas at all is discarded.
func loadQuotas(ctx context.Context, executor GraphQLExecutor) (*registryQuotas, error) {
	req := graphql.NewRequest(queryRegistryQuotas)

	var resp registryQuotasResponse
	err := executor.Execute(ctx, req, &resp)
	if err != nil {
		if isEmptyQuotas(resp.RegistryQuotas) {
			return nil, err
		}
		return &resp.RegistryQuotas, err
	}

	return &resp.RegistryQuotas, nil
}

func isEmptyQuotas(quotas registryQuotas) bool {
	return quotas.PrivateRegistry == nil && len(quotas.OnchainRegistries) == 0
}

// remaining clamps at zero: a family can be disabled (limit 0) while still running workflows, and
// that must render as no headroom rather than a negative number.
func remaining(limit, used int) int {
	if used >= limit {
		return 0
	}
	return limit - used
}

func findPrivateQuota(quotas *registryQuotas) *privateRegistryQuota {
	if quotas == nil {
		return nil
	}
	return quotas.PrivateRegistry
}

func findOnchainQuota(quotas *registryQuotas, registry *tenantctx.Registry) *onchainRegistryQuota {
	if quotas == nil || registry == nil || registry.ChainSelector == nil || registry.Address == nil {
		return nil
	}

	for i := range quotas.OnchainRegistries {
		candidate := &quotas.OnchainRegistries[i]
		if candidate.ChainSelector == *registry.ChainSelector &&
			strings.EqualFold(candidate.Address, *registry.Address) {
			return candidate
		}
	}

	return nil
}

func renderWorkflows(workflows []quotaWorkflow) {
	if len(workflows) == 0 {
		return
	}

	ui.Dim("  Workflows:")
	for _, workflow := range workflows {
		if workflow.Limit == 0 {
			ui.Dim(fmt.Sprintf("    %-16s %d / 0 used        no headroom", workflow.DonFamily, workflow.Active))
			continue
		}
		ui.Dim(fmt.Sprintf("    %-16s %d / %d used       %d remaining",
			workflow.DonFamily, workflow.Active, workflow.Limit, remaining(workflow.Limit, workflow.Active)))
	}
}

func renderLinkedOwners(quota *onchainRegistryQuota) {
	if quota == nil {
		return
	}

	used := len(quota.LinkedOwners.Linked) + len(quota.LinkedOwners.Pending)
	ui.Dim(fmt.Sprintf("  Linked owners:   %d / %d used       %d remaining",
		used, quota.LinkedOwners.Limit, remaining(quota.LinkedOwners.Limit, used)))

	for _, owner := range quota.LinkedOwners.Linked {
		ui.Dim(fmt.Sprintf("    - %s", owner))
	}
	for _, owner := range quota.LinkedOwners.Pending {
		ui.Dim(fmt.Sprintf("    - %s (pending)", owner))
	}
}

func renderOwners(quota *onchainRegistryQuota) {
	if quota == nil || len(quota.Owners) == 0 {
		return
	}

	for _, owner := range quota.Owners {
		ui.Dim(fmt.Sprintf("    %s", owner.Owner))
		for _, workflow := range owner.Workflows {
			if workflow.Limit == 0 {
				ui.Dim(fmt.Sprintf("      %-14s %d / 0 used      no headroom", workflow.DonFamily, workflow.Active))
				continue
			}
			ui.Dim(fmt.Sprintf("      %-14s %d / %d used     %d remaining",
				workflow.DonFamily, workflow.Active, workflow.Limit, remaining(workflow.Limit, workflow.Active)))
		}
	}
}

package limits

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/registrytype"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

type handler struct {
	runtimeContext *runtime.Context
	executor       GraphQLExecutor
}

// New returns the `cre registry limits` command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	return newWithExecutor(runtimeContext, nil)
}

func newWithExecutor(runtimeContext *runtime.Context, executor GraphQLExecutor) *cobra.Command {
	h := &handler{runtimeContext: runtimeContext, executor: executor}

	return &cobra.Command{
		Use:   "limits",
		Short: "Shows workflow usage and limits for the current registries",
		Long: `Displays how many workflows each registry is running against its limit.

For the private registry, limits are per DON family. For an on-chain registry, the linked-owner
quota is shown first, followed by the on-chain workflow count and limit for each linked owner.

Usage is fetched live on every run; it is not read from the cached tenant context.`,
		Example: `cre registry limits`,
		Args:    cobra.NoArgs,
		RunE:    h.run,
	}
}

// executorFor resolves the GraphQL client lazily so constructing the command never depends on
// credentials or an environment being present.
func (h *handler) executorFor() GraphQLExecutor {
	if h.executor != nil {
		return h.executor
	}
	if h.runtimeContext.EnvironmentSet == nil {
		return nil
	}
	return graphqlclient.New(
		h.runtimeContext.Credentials,
		h.runtimeContext.EnvironmentSet,
		h.runtimeContext.Logger,
	)
}

func (h *handler) run(cmd *cobra.Command, _ []string) error {
	if h.runtimeContext.TenantContext == nil {
		return fmt.Errorf("user context not available — run `cre login` and retry")
	}

	registries := h.runtimeContext.TenantContext.Registries
	if len(registries) == 0 {
		ui.Warning("No registries found for this environment")
		return nil
	}

	executor := h.executorFor()
	if executor == nil {
		return fmt.Errorf("user context not available — run `cre login` and retry")
	}

	quotas, quotaErr := loadQuotas(cmd.Context(), executor)
	if quotas == nil {
		return quotaErr
	}

	ui.Line()
	ui.Bold("Registry limits")
	ui.Line()

	rendered := false
	for _, registry := range registries {
		switch registrytype.Type(registry.Type) {
		case registrytype.OffChain:
			if private := findPrivateQuota(quotas); private != nil {
				ui.Bold(registry.Label)
				renderWorkflows(private.Workflows)
				ui.Line()
				rendered = true
			}
		case registrytype.OnChain:
			if onchain := findOnchainQuota(quotas, registry); onchain != nil {
				ui.Bold(registry.Label)
				renderLinkedOwners(onchain)
				renderOwners(onchain)
				ui.Line()
				rendered = true
			}
		default:
			// Unknown types have no quota shape to render.
		}
	}

	if quotaErr != nil {
		ui.Warning(fmt.Sprintf("Registry limits are incomplete: %s", quotaErr.Error()))
	}
	if !rendered && quotaErr == nil {
		ui.Warning("No registry limits available")
	}

	return nil
}

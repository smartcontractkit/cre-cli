package http

import (
	"context"
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

// resolveWorkflowID looks up the deployed workflow ID for the workflow-name configured under
// the selected --target, scoped the same way `cre workflow get` scopes it.
func (h *Handler) resolveWorkflowID(ctx context.Context) (string, error) {
	rt := h.runtimeContext
	if rt.TenantContext == nil {
		return "", fmt.Errorf("user context not available — run `cre login` and retry")
	}

	name := strings.TrimSpace(rt.Settings.Workflow.UserWorkflowSettings.WorkflowName)
	if name == "" {
		return "", fmt.Errorf("workflow-name is not set for target %q in workflow.yaml", rt.Settings.User.TargetName)
	}

	owner, err := workflowresolve.ResolveWorkflowOwnerAddress(rt.Settings, rt.ResolvedRegistry, rt.DerivedWorkflowOwner)
	if err != nil {
		return "", err
	}

	registry, err := h.registryFilter()
	if err != nil {
		return "", err
	}

	wdc := workflowdataclient.New(graphqlclient.New(rt.Credentials, rt.EnvironmentSet, rt.Logger), rt.Logger)

	spinner := ui.NewSpinner()
	spinner.Start(fmt.Sprintf("Resolving workflow %q...", name))
	rows, err := wdc.SearchByName(ctx, name, workflowdataclient.DefaultPageSize, owner)
	spinner.Stop()
	if err != nil {
		return "", fmt.Errorf("resolving workflow name %q: %w", name, err)
	}

	rows = workflowresolve.OmitDeleted(exactNameMatches(rows, name))
	if registry != nil {
		rows = workflowresolve.FilterRowsByRegistry(rows, registry, rt.TenantContext.Registries)
	}

	row, err := pickWorkflow(rows, name)
	if err != nil {
		return "", err
	}
	return normalizeWorkflowID(row.WorkflowID)
}

func (h *Handler) registryFilter() (*tenantctx.Registry, error) {
	rt := h.runtimeContext
	id := strings.TrimSpace(rt.Settings.Workflow.UserWorkflowSettings.DeploymentRegistry)
	if id == "" && rt.ResolvedRegistry != nil {
		id = rt.ResolvedRegistry.ID()
	}
	if id == "" {
		return nil, nil
	}

	registry := workflowresolve.FindRegistry(rt.TenantContext.Registries, id)
	if registry == nil {
		return nil, fmt.Errorf("deployment-registry %q not found in user context; available: [%s]",
			id, workflowresolve.AvailableRegistryIDs(rt.TenantContext.Registries))
	}
	return registry, nil
}

func exactNameMatches(rows []workflowresolve.Workflow, name string) []workflowresolve.Workflow {
	out := make([]workflowresolve.Workflow, 0, len(rows))
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Name), name) {
			out = append(out, r)
		}
	}
	return out
}

func pickWorkflow(rows []workflowresolve.Workflow, name string) (workflowresolve.Workflow, error) {
	if len(rows) == 0 {
		return workflowresolve.Workflow{}, fmt.Errorf("no workflow found with name %q; deploy it first or pass --workflow-id", name)
	}

	var active []workflowresolve.Workflow
	for _, r := range rows {
		if strings.EqualFold(r.Status, "ACTIVE") {
			active = append(active, r)
		}
	}

	switch {
	case len(active) == 1:
		return active[0], nil
	case len(active) > 1:
		return workflowresolve.Workflow{}, fmt.Errorf("multiple ACTIVE workflows named %q found; pass --workflow-id to pick one", name)
	default:
		ui.Warning(fmt.Sprintf("No ACTIVE deployment for workflow %q; using the first match (status: %s)", name, rows[0].Status))
		return rows[0], nil
	}
}

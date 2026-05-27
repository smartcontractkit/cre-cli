package get

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/workflowrender"
)

// Handler resolves a single workflow by name via the platform search API and
// prints the matching rows. It filters to the workflow's configured
// deployment-registry by default; that filter can be disabled with --all-registries.
type Handler struct {
	credentials      *credentials.Credentials
	tenantCtx        *tenantctx.EnvironmentContext
	settings         *settings.Settings
	resolvedRegistry settings.ResolvedRegistry
	wdc              *workflowdataclient.Client
}

// NewHandler builds a Handler backed by a real WorkflowDataClient.
func NewHandler(ctx *runtime.Context) *Handler {
	gql := graphqlclient.New(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
	wdc := workflowdataclient.New(gql, ctx.Logger)
	return &Handler{
		credentials:      ctx.Credentials,
		tenantCtx:        ctx.TenantContext,
		settings:         ctx.Settings,
		resolvedRegistry: ctx.ResolvedRegistry,
		wdc:              wdc,
	}
}

// NewHandlerWithClient builds a Handler with a pre-built WorkflowDataClient
// (for testing).
func NewHandlerWithClient(ctx *runtime.Context, wdc *workflowdataclient.Client) *Handler {
	return &Handler{
		credentials:      ctx.Credentials,
		tenantCtx:        ctx.TenantContext,
		settings:         ctx.Settings,
		resolvedRegistry: ctx.ResolvedRegistry,
		wdc:              wdc,
	}
}

// Execute searches the platform for workflows whose name matches the value in
// the target's user-workflow settings. When allRegistries is false (default),
// results are filtered to the workflow's configured deployment-registry.
func (h *Handler) Execute(ctx context.Context, allRegistries bool) error {
	if h.tenantCtx == nil {
		return fmt.Errorf("user context not available — run `cre login` and retry")
	}
	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}
	if h.settings == nil {
		return fmt.Errorf("workflow settings not loaded; ensure workflow.yaml is valid")
	}

	workflowName := strings.TrimSpace(h.settings.Workflow.UserWorkflowSettings.WorkflowName)
	if workflowName == "" {
		return fmt.Errorf("workflow-name is not set for target %q in workflow.yaml", h.settings.User.TargetName)
	}

	// Resolve which registry to filter by (if any). When --all-registries is
	// set we skip the filter entirely. Otherwise we use the deployment-registry
	// from workflow.yaml; the registry-resolution pass that runs earlier
	// already validated the value against the tenant context, so we look it
	// up by ID here for a plain *tenantctx.Registry handle to pass to the
	// filter.
	var registryFilter *tenantctx.Registry
	if !allRegistries {
		filterID := strings.TrimSpace(h.settings.Workflow.UserWorkflowSettings.DeploymentRegistry)
		if filterID == "" && h.resolvedRegistry != nil {
			filterID = h.resolvedRegistry.ID()
		}
		if filterID != "" {
			registryFilter = workflowrender.FindRegistry(h.tenantCtx.Registries, filterID)
			if registryFilter == nil {
				return fmt.Errorf("deployment-registry %q not found in user context; available: [%s]",
					filterID, workflowrender.AvailableRegistryIDs(h.tenantCtx.Registries))
			}
		}
	}

	spinner := ui.NewSpinner()
	spinner.Start(fmt.Sprintf("Fetching workflow %q...", workflowName))
	rows, err := h.wdc.SearchByName(ctx, workflowName, workflowdataclient.DefaultPageSize)
	spinner.Stop()
	if err != nil {
		return err
	}

	// The platform search uses a contains-style match; narrow to an exact
	// (case-insensitive) name match so `get foo` does not surface `foo-staging`.
	rows = filterByExactName(rows, workflowName)

	if registryFilter != nil {
		rows = workflowrender.FilterRowsByRegistry(rows, registryFilter, h.tenantCtx.Registries)
	}

	workflowrender.PrintWorkflowTable(rows, h.tenantCtx.Registries, workflowrender.TableOptions{})
	return nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var allRegistries bool

	cmd := &cobra.Command{
		Use:   "get <workflow-folder-path>",
		Short: "Shows metadata for the workflow configured in workflow.yaml",
		Long: `Looks up the workflow whose name is configured for the selected --target in ` +
			`workflow.yaml and prints its metadata from the CRE platform. By default results ` +
			`are filtered to the workflow's configured deployment-registry; pass --all-registries ` +
			`to show matches from every registry.`,
		Example: `cre workflow get ./my-workflow --target staging
  cre workflow get ./my-workflow --target staging --all-registries`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return NewHandler(runtimeContext).Execute(cmd.Context(), allRegistries)
		},
	}

	cmd.Flags().BoolVar(&allRegistries, "all-registries", false,
		"Do not filter results by the workflow's deployment-registry")

	return cmd
}

func filterByExactName(rows []workflowrender.Workflow, name string) []workflowrender.Workflow {
	out := make([]workflowrender.Workflow, 0, len(rows))
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Name), name) {
			out = append(out, r)
		}
	}
	return out
}

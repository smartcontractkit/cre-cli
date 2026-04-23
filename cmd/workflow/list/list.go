package list

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// Workflow is a type alias so that print.go and registry.go in this package
// can use the name without importing workflowdataclient directly.
type Workflow = workflowdataclient.Workflow

// Handler loads workflows via the WorkflowDataClient and prints them.
type Handler struct {
	credentials *credentials.Credentials
	tenantCtx   *tenantctx.EnvironmentContext
	wdc         *workflowdataclient.Client
}

// NewHandler builds a Handler with a real WorkflowDataClient.
func NewHandler(ctx *runtime.Context) *Handler {
	gql := graphqlclient.New(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
	wdc := workflowdataclient.New(gql, ctx.Logger)
	return &Handler{
		credentials: ctx.Credentials,
		tenantCtx:   ctx.TenantContext,
		wdc:         wdc,
	}
}

// NewHandlerWithClient builds a Handler with a pre-built WorkflowDataClient (for testing).
func NewHandlerWithClient(ctx *runtime.Context, wdc *workflowdataclient.Client) *Handler {
	return &Handler{
		credentials: ctx.Credentials,
		tenantCtx:   ctx.TenantContext,
		wdc:         wdc,
	}
}

// Execute lists workflows, optionally filtering by registry ID from user context.
// Deleted workflows are omitted unless includeDeleted is true.
func (h *Handler) Execute(ctx context.Context, registryFilter string, includeDeleted bool) error {
	if h.tenantCtx == nil {
		return fmt.Errorf("user context not available — run `cre login` and retry")
	}

	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	if registryFilter != "" {
		if findRegistry(h.tenantCtx.Registries, registryFilter) == nil {
			return fmt.Errorf("registry %q not found in user context; available: [%s]",
				registryFilter, availableRegistryIDs(h.tenantCtx.Registries))
		}
	}

	spinner := ui.NewSpinner()
	spinner.Start("Listing workflows...")
	rows, err := h.wdc.ListAll(ctx, workflowdataclient.DefaultPageSize)
	spinner.Stop()
	if err != nil {
		return err
	}

	if registryFilter != "" {
		reg := findRegistry(h.tenantCtx.Registries, registryFilter)
		rows = filterRowsByRegistry(rows, reg, h.tenantCtx.Registries)
	}

	afterRegistryFilter := len(rows)
	if !includeDeleted {
		rows = omitDeleted(rows)
	}

	printWorkflowTable(rows, h.tenantCtx.Registries, afterRegistryFilter, includeDeleted)
	return nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var registryID string
	var includeDeleted bool

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "Lists workflows deployed for your organization",
		Long:    `Lists workflows across registries in your organization. Requires authentication and user context. Deleted workflows are hidden by default.`,
		Example: "cre workflow list\n  cre workflow list --registry private\n  cre workflow list --include-deleted",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return NewHandler(runtimeContext).Execute(cmd.Context(), registryID, includeDeleted)
		},
	}

	cmd.Flags().StringVar(&registryID, "registry", "", "Filter by registry ID from user context")
	cmd.Flags().BoolVar(&includeDeleted, "include-deleted", false, "Include workflows in DELETED status")
	return cmd
}

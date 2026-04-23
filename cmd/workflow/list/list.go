package list

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

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

// Inputs holds the resolved and validated flag values for the list command.
type Inputs struct {
	RegistryFilter string
	IncludeDeleted bool
	// OutputPath is an absolute path to a .json file, or "" if not set.
	OutputPath string
}

// resolveInputs builds Inputs from raw flag values, resolving the output path
// to an absolute path and validating that it carries a .json extension.
func resolveInputs(registryFilter string, includeDeleted bool, outputPath string) (Inputs, error) {
	resolved := Inputs{
		RegistryFilter: registryFilter,
		IncludeDeleted: includeDeleted,
	}

	if outputPath != "" {
		if ext := strings.ToLower(filepath.Ext(outputPath)); ext != ".json" {
			return Inputs{}, fmt.Errorf("--output must use a .json extension, got %q", outputPath)
		}
		abs, err := filepath.Abs(outputPath)
		if err != nil {
			return Inputs{}, fmt.Errorf("cannot resolve --output path %q: %w", outputPath, err)
		}
		resolved.OutputPath = abs
	}

	return resolved, nil
}

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

// Execute lists workflows applying the filters from inputs.
// Deleted workflows are omitted unless inputs.IncludeDeleted is true.
// When inputs.OutputPath is set, results are also written as JSON to that file.
func (h *Handler) Execute(ctx context.Context, inputs Inputs) error {
	if h.tenantCtx == nil {
		return fmt.Errorf("user context not available — run `cre login` and retry")
	}

	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	if inputs.RegistryFilter != "" {
		if findRegistry(h.tenantCtx.Registries, inputs.RegistryFilter) == nil {
			return fmt.Errorf("registry %q not found in user context; available: [%s]",
				inputs.RegistryFilter, availableRegistryIDs(h.tenantCtx.Registries))
		}
	}

	spinner := ui.NewSpinner()
	spinner.Start("Listing workflows...")
	rows, err := h.wdc.ListAll(ctx, workflowdataclient.DefaultPageSize)
	spinner.Stop()
	if err != nil {
		return err
	}

	if inputs.RegistryFilter != "" {
		reg := findRegistry(h.tenantCtx.Registries, inputs.RegistryFilter)
		rows = filterRowsByRegistry(rows, reg, h.tenantCtx.Registries)
	}

	afterRegistryFilter := len(rows)
	if !inputs.IncludeDeleted {
		rows = omitDeleted(rows)
	}

	printWorkflowTable(rows, h.tenantCtx.Registries, afterRegistryFilter, inputs.IncludeDeleted)

	if inputs.OutputPath != "" {
		if err := writeWorkflowsJSON(rows, h.tenantCtx.Registries, inputs.OutputPath); err != nil {
			return fmt.Errorf("write JSON output: %w", err)
		}
		ui.Success(fmt.Sprintf("Results written to %s", inputs.OutputPath))
	}

	return nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var registryID string
	var includeDeleted bool
	var outputPath string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists workflows deployed for your organization",
		Long:  `Lists workflows across registries in your organization. Requires authentication and user context. Deleted workflows are hidden by default.`,
		Example: "cre workflow list\n" +
			"  cre workflow list --registry private\n" +
			"  cre workflow list --include-deleted\n" +
			"  cre workflow list --output /path/to/workflows.json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := resolveInputs(registryID, includeDeleted, outputPath)
			if err != nil {
				return err
			}
			return NewHandler(runtimeContext).Execute(cmd.Context(), inputs)
		},
	}

	cmd.Flags().StringVar(&registryID, "registry", "", "Filter by registry ID from user context")
	cmd.Flags().BoolVar(&includeDeleted, "include-deleted", false, "Include workflows in DELETED status")
	cmd.Flags().StringVar(&outputPath, "output", "", "Write results to a .json file at the given path (relative or absolute)")
	return cmd
}

package get

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

// Inputs holds resolved and validated flag values for workflow get.
type Inputs struct {
	AllRegistries bool
	OutputFormat  string
}

func resolveInputs(allRegistries bool, outputFormat string, jsonFlag bool) (Inputs, error) {
	outputFormat, err := workflowresolve.ResolveOutputFormat(outputFormat, jsonFlag)
	if err != nil {
		return Inputs{}, err
	}
	return Inputs{
		AllRegistries: allRegistries,
		OutputFormat:  outputFormat,
	}, nil
}

// Handler resolves a single workflow by name via the platform search API and
// prints deployment health and the most recent execution. Results are filtered
// to the workflow's configured deployment-registry by default; pass
// --all-registries to resolve across every registry.
type Handler struct {
	credentials          *credentials.Credentials
	tenantCtx            *tenantctx.EnvironmentContext
	settings             *settings.Settings
	resolvedRegistry     settings.ResolvedRegistry
	derivedWorkflowOwner string
	wdc                  *workflowdataclient.Client
}

// NewHandler builds a Handler backed by a real WorkflowDataClient.
func NewHandler(ctx *runtime.Context) *Handler {
	gql := graphqlclient.New(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
	wdc := workflowdataclient.New(gql, ctx.Logger)
	return &Handler{
		credentials:          ctx.Credentials,
		tenantCtx:            ctx.TenantContext,
		settings:             ctx.Settings,
		resolvedRegistry:     ctx.ResolvedRegistry,
		derivedWorkflowOwner: ctx.DerivedWorkflowOwner,
		wdc:                  wdc,
	}
}

// NewHandlerWithClient builds a Handler with a pre-built WorkflowDataClient
// (for testing).
func NewHandlerWithClient(ctx *runtime.Context, wdc *workflowdataclient.Client) *Handler {
	return &Handler{
		credentials:          ctx.Credentials,
		tenantCtx:            ctx.TenantContext,
		settings:             ctx.Settings,
		resolvedRegistry:     ctx.ResolvedRegistry,
		derivedWorkflowOwner: ctx.DerivedWorkflowOwner,
		wdc:                  wdc,
	}
}

// Execute resolves the workflow configured for the selected --target and prints
// deployment health and the most recent execution.
func (h *Handler) Execute(ctx context.Context, inputs Inputs) error {
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

	owner, err := workflowresolve.ResolveWorkflowOwnerAddress(h.settings, h.resolvedRegistry, h.derivedWorkflowOwner)
	if err != nil {
		return err
	}

	uuid, err := h.resolveWorkflowUUID(ctx, inputs, workflowName, owner)
	if err != nil {
		return err
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching workflow details...")

	now := time.Now().UTC()
	from := now.AddDate(-1, 0, 0) // 1-year lookback — mirrors Explorer behaviour

	var (
		summary                        *workflowdataclient.WorkflowSummary
		deployment                     *workflowdataclient.WorkflowDeploymentRecord
		executions                     []workflowdataclient.Execution
		summaryErr, deployErr, execErr error
		wg                             sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		summary, summaryErr = h.wdc.GetWorkflowSummary(ctx, uuid, from)
	}()
	go func() {
		defer wg.Done()
		deployment, deployErr = h.wdc.GetLatestDeployment(ctx, uuid, from, now)
	}()
	go func() {
		defer wg.Done()
		executions, execErr = h.wdc.ListExecutions(ctx, workflowdataclient.ListExecutionsInput{
			WorkflowUUID: &uuid,
			Limit:        1,
		})
	}()
	wg.Wait()
	spinner.Stop()

	if summaryErr != nil {
		return summaryErr
	}
	if execErr != nil {
		return execErr
	}
	if deployErr != nil {
		deployment = nil
		ui.Warning(fmt.Sprintf("Could not fetch deployment record: %s", deployErr.Error()))
	}

	var lastExec *workflowdataclient.Execution
	if len(executions) > 0 {
		lastExec = &executions[0]
	}

	view := workflowresolve.WorkflowStatusView{
		Summary:       summary,
		Deployment:    deployment,
		DeploymentErr: deployErr,
		LastExecution: lastExec,
		Registries:    h.tenantCtx.Registries,
	}

	if inputs.OutputFormat == workflowresolve.OutputFormatJSON {
		return workflowresolve.PrintWorkflowStatusJSON(view)
	}
	workflowresolve.PrintWorkflowStatusTable(view)
	return nil
}

func (h *Handler) registryFilter(inputs Inputs) (*tenantctx.Registry, error) {
	if inputs.AllRegistries {
		return nil, nil
	}
	filterID := strings.TrimSpace(h.settings.Workflow.UserWorkflowSettings.DeploymentRegistry)
	if filterID == "" && h.resolvedRegistry != nil {
		filterID = h.resolvedRegistry.ID()
	}
	if filterID == "" {
		return nil, nil
	}
	registryFilter := workflowresolve.FindRegistry(h.tenantCtx.Registries, filterID)
	if registryFilter == nil {
		return nil, fmt.Errorf("deployment-registry %q not found in user context; available: [%s]",
			filterID, workflowresolve.AvailableRegistryIDs(h.tenantCtx.Registries))
	}
	return registryFilter, nil
}

func (h *Handler) resolveWorkflowUUID(ctx context.Context, inputs Inputs, workflowName, owner string) (string, error) {
	registryFilter, err := h.registryFilter(inputs)
	if err != nil {
		return "", err
	}

	spinner := ui.NewSpinner()
	spinner.Start(fmt.Sprintf("Resolving workflow %q...", workflowName))
	rows, err := h.wdc.SearchByName(ctx, workflowName, workflowdataclient.DefaultPageSize, owner)
	spinner.Stop()
	if err != nil {
		return "", fmt.Errorf("resolving workflow name %q: %w", workflowName, err)
	}

	rows = filterByExactName(rows, workflowName)
	if registryFilter != nil {
		rows = workflowresolve.FilterRowsByRegistry(rows, registryFilter, h.tenantCtx.Registries)
	}

	if len(rows) == 0 {
		return "", fmt.Errorf("no workflow found with name %q", workflowName)
	}

	var active []workflowresolve.Workflow
	for _, r := range rows {
		if strings.EqualFold(r.Status, "ACTIVE") {
			active = append(active, r)
		}
	}
	if len(active) == 1 {
		if active[0].UUID == "" {
			return "", fmt.Errorf("workflow %q resolved but has no platform UUID", workflowName)
		}
		return active[0].UUID, nil
	}
	if len(active) > 1 {
		return "", fmt.Errorf("multiple ACTIVE workflows named %q found; narrow the deployment-registry in workflow.yaml or pass --all-registries", workflowName)
	}

	if rows[0].UUID == "" {
		return "", fmt.Errorf("workflow %q resolved but has no platform UUID", workflowName)
	}
	ui.Warning(fmt.Sprintf("No ACTIVE deployment for workflow %q; using the first match (status: %s)", workflowName, rows[0].Status))
	return rows[0].UUID, nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var allRegistries bool
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "get <workflow-folder-path>",
		Short: "Show deployment health and recent execution for the workflow in workflow.yaml",
		Long: `Looks up the workflow whose name is configured for the selected --target in ` +
			`workflow.yaml and prints deployment health and the most recent execution from ` +
			`the CRE platform. By default resolution is scoped to the workflow's configured ` +
			`deployment-registry; pass --all-registries to resolve across every registry.`,
		Example: `cre workflow get ./my-workflow --target staging
  cre workflow get ./my-workflow --target staging --all-registries
  cre workflow get ./my-workflow --target staging --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := resolveInputs(allRegistries, outputFormat, jsonFlag)
			if err != nil {
				return err
			}
			return NewHandler(runtimeContext).Execute(cmd.Context(), inputs)
		},
	}

	cmd.Flags().BoolVar(&allRegistries, "all-registries", false,
		"Resolve the workflow across every registry instead of the configured deployment-registry")
	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints JSON to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")
	return cmd
}

func filterByExactName(rows []workflowresolve.Workflow, name string) []workflowresolve.Workflow {
	out := make([]workflowresolve.Workflow, 0, len(rows))
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Name), name) {
			out = append(out, r)
		}
	}
	return out
}

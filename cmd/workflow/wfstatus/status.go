// Package wfstatus implements the `cre workflow status` command.
// It is named wfstatus to avoid a collision with the Go standard library
// package name "status" in import paths.
package wfstatus

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

// Inputs holds resolved and validated flag/arg values for workflow status.
type Inputs struct {
	OutputFormat string
}

func resolveInputs(outputFormat string, jsonFlag bool) (Inputs, error) {
	outputFormat, err := workflowresolve.ResolveOutputFormat(outputFormat, jsonFlag)
	if err != nil {
		return Inputs{}, err
	}
	return Inputs{OutputFormat: outputFormat}, nil
}

// Handler fetches and renders a comprehensive workflow status view.
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

// NewHandlerWithClient builds a Handler with a pre-built client (for testing).
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

// Execute fetches all status data in parallel and renders it.
func (h *Handler) Execute(ctx context.Context, inputs Inputs) error {
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

	uuid, err := workflowresolve.ResolveWorkflowUUID(ctx, h.wdc, workflowName, workflowresolve.ResolveOptions{
		WorkflowOwnerAddress: owner,
	})
	if err != nil {
		return err
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching workflow status...")

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

	var registries []*tenantctx.Registry
	if h.tenantCtx != nil {
		registries = h.tenantCtx.Registries
	}

	view := workflowresolve.WorkflowStatusView{
		Summary:       summary,
		Deployment:    deployment,
		DeploymentErr: deployErr,
		LastExecution: lastExec,
		Registries:    registries,
	}

	if inputs.OutputFormat == workflowresolve.OutputFormatJSON {
		return workflowresolve.PrintWorkflowStatusJSON(view)
	}
	workflowresolve.PrintWorkflowStatusTable(view)
	return nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "status <workflow-folder-path>",
		Short: "Show deployment health and execution summary for a workflow",
		Long: `Show the full health picture of the workflow configured for the selected
--target in workflow.yaml: deployment status, activation state, execution
success/failure counts, and the most recent execution.

Useful for diagnosing the gap between registering a workflow and it
becoming active in the DON, or for a quick health check.`,
		Example: "cre workflow status ./my-workflow --target staging\n" +
			"  cre workflow status ./my-workflow --target staging --output json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := resolveInputs(outputFormat, jsonFlag)
			if err != nil {
				return err
			}
			return NewHandler(runtimeContext).Execute(cmd.Context(), inputs)
		},
	}

	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints JSON to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")
	return cmd
}

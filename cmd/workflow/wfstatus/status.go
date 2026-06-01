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
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/workflowrender"
)

const outputFormatJSON = "json"

// Handler fetches and renders a comprehensive workflow status view.
type Handler struct {
	credentials *credentials.Credentials
	wdc         *workflowdataclient.Client
}

// NewHandler builds a Handler backed by a real WorkflowDataClient.
func NewHandler(ctx *runtime.Context) *Handler {
	gql := graphqlclient.New(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
	wdc := workflowdataclient.New(gql, ctx.Logger)
	return &Handler{credentials: ctx.Credentials, wdc: wdc}
}

// NewHandlerWithClient builds a Handler with a pre-built client (for testing).
func NewHandlerWithClient(ctx *runtime.Context, wdc *workflowdataclient.Client) *Handler {
	return &Handler{credentials: ctx.Credentials, wdc: wdc}
}

// resolveUUID returns the platform UUID for a workflow name or on-chain WorkflowID.
func (h *Handler) resolveUUID(ctx context.Context, arg string) (string, error) {
	if looksLikeWorkflowID(arg) {
		return h.resolveByWorkflowID(ctx, arg)
	}
	return h.resolveByName(ctx, arg)
}

func (h *Handler) resolveByWorkflowID(ctx context.Context, workflowID string) (string, error) {
	spinner := ui.NewSpinner()
	spinner.Start(fmt.Sprintf("Resolving workflow ID %q...", workflowID))
	rows, err := h.wdc.ListAll(ctx, workflowdataclient.DefaultPageSize)
	spinner.Stop()
	if err != nil {
		return "", fmt.Errorf("resolving workflow ID %q: %w", workflowID, err)
	}
	for _, r := range rows {
		if strings.EqualFold(r.WorkflowID, workflowID) {
			if r.UUID == "" {
				return "", fmt.Errorf("workflow with ID %q found but has no platform UUID", workflowID)
			}
			return r.UUID, nil
		}
	}
	return "", fmt.Errorf("no workflow found with ID %q", workflowID)
}

func (h *Handler) resolveByName(ctx context.Context, name string) (string, error) {
	spinner := ui.NewSpinner()
	spinner.Start(fmt.Sprintf("Resolving workflow %q...", name))
	rows, err := h.wdc.SearchByName(ctx, name, workflowdataclient.DefaultPageSize)
	spinner.Stop()
	if err != nil {
		return "", fmt.Errorf("resolving workflow name %q: %w", name, err)
	}
	var matches []workflowdataclient.Workflow
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Name), name) {
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no workflow found with name %q", name)
	}
	for _, r := range matches {
		if strings.EqualFold(r.Status, "ACTIVE") {
			return r.UUID, nil
		}
	}
	return matches[0].UUID, nil
}

// Execute fetches all status data in parallel and renders it.
func (h *Handler) Execute(ctx context.Context, arg, outputFormat string) error {
	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	uuid, err := h.resolveUUID(ctx, arg)
	if err != nil {
		return err
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching workflow status...")

	from := time.Now().UTC().AddDate(0, 0, -30)

	var (
		summary                                          *workflowdataclient.WorkflowSummary
		deployment                                       *workflowdataclient.WorkflowDeploymentRecord
		executions                                       []workflowdataclient.Execution
		successCount, failureCount                       int
		summaryErr, deployErr, execErr, succErr, failErr error
		wg                                               sync.WaitGroup
	)

	wg.Add(5)
	go func() {
		defer wg.Done()
		summary, summaryErr = h.wdc.GetWorkflowSummary(ctx, uuid, from)
	}()
	go func() {
		defer wg.Done()
		deployment, deployErr = h.wdc.GetLatestDeployment(ctx, uuid)
	}()
	go func() {
		defer wg.Done()
		executions, execErr = h.wdc.ListExecutions(ctx, workflowdataclient.ListExecutionsInput{
			WorkflowUUID: &uuid,
			Limit:        1,
		})
	}()
	go func() {
		defer wg.Done()
		successCount, succErr = h.wdc.CountExecutions(ctx, uuid, []workflowdataclient.ExecutionStatus{workflowdataclient.ExecutionStatusSuccess})
	}()
	go func() {
		defer wg.Done()
		failureCount, failErr = h.wdc.CountExecutions(ctx, uuid, []workflowdataclient.ExecutionStatus{workflowdataclient.ExecutionStatusFailure})
	}()
	wg.Wait()
	spinner.Stop()

	if summaryErr != nil {
		return summaryErr
	}
	if execErr != nil {
		return execErr
	}
	// deployErr, succErr, failErr are non-fatal — degrade gracefully.
	if deployErr != nil {
		deployment = nil
	}
	if succErr != nil {
		successCount = 0
	}
	if failErr != nil {
		failureCount = 0
	}
	summary.SuccessCount = successCount
	summary.FailureCount = failureCount
	summary.ExecutionCount = successCount + failureCount

	var lastExec *workflowdataclient.Execution
	if len(executions) > 0 {
		lastExec = &executions[0]
	}

	view := workflowrender.WorkflowStatusView{
		Summary:       summary,
		Deployment:    deployment,
		LastExecution: lastExec,
	}

	if outputFormat == outputFormatJSON {
		return workflowrender.PrintWorkflowStatusJSON(view)
	}
	workflowrender.PrintWorkflowStatusTable(view)
	return nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "status <workflow-id-or-name>",
		Short: "Show deployment health and execution summary for a workflow",
		Long: `Show the full health picture of a workflow: deployment status, activation
state, execution success/failure counts, and the most recent execution.

Useful for diagnosing the gap between registering a workflow and it
becoming active in the DON, or for a quick health check.`,
		Example: "cre workflow status my-workflow\n" +
			"  cre workflow status 00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856\n" +
			"  cre workflow status my-workflow --output json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonFlag {
				outputFormat = outputFormatJSON
			}
			if outputFormat != "" && outputFormat != outputFormatJSON {
				return fmt.Errorf("--output %q is not supported; only %q is accepted", outputFormat, outputFormatJSON)
			}
			return NewHandler(runtimeContext).Execute(cmd.Context(), args[0], outputFormat)
		},
	}

	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints JSON to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")
	return cmd
}

// looksLikeWorkflowID returns true for 64-char hex strings (on-chain WorkflowID).
func looksLikeWorkflowID(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

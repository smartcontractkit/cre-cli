package execution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/workflowrender"
)

const outputFormatJSON = "json"

// ListInputs holds the resolved and validated flag/arg values for `execution list`.
type ListInputs struct {
	// WorkflowUUID is the resolved platform UUID to filter by, or nil for all executions.
	WorkflowUUID *string
	Statuses     []workflowdataclient.ExecutionStatus
	From         *time.Time
	To           *time.Time
	Limit        int
	OutputFormat string
}

// ListHandler fetches and renders a list of workflow executions.
type ListHandler struct {
	credentials    *credentials.Credentials
	wdc            *workflowdataclient.Client
	settings       *settings.Settings
	nonInteractive bool
}

func newListHandler(ctx *runtime.Context) *ListHandler {
	gql := graphqlclient.New(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
	wdc := workflowdataclient.New(gql, ctx.Logger)
	nonInteractive := false
	if ctx.Viper != nil {
		nonInteractive = ctx.Viper.GetBool(settings.Flags.NonInteractive.Name)
	}
	return &ListHandler{
		credentials:    ctx.Credentials,
		wdc:            wdc,
		settings:       ctx.Settings,
		nonInteractive: nonInteractive,
	}
}

// NewListHandlerWithClient builds a ListHandler with a pre-built client (for testing).
func NewListHandlerWithClient(ctx *runtime.Context, wdc *workflowdataclient.Client) *ListHandler {
	nonInteractive := false
	if ctx.Viper != nil {
		nonInteractive = ctx.Viper.GetBool(settings.Flags.NonInteractive.Name)
	}
	return &ListHandler{
		credentials:    ctx.Credentials,
		wdc:            wdc,
		settings:       ctx.Settings,
		nonInteractive: nonInteractive,
	}
}

// resolveListInputs validates and resolves flag values into ListInputs.
func (h *ListHandler) resolveListInputs(
	_ context.Context,
	statusFlag, startFlag, endFlag string,
	limit int,
	outputFormat string,
) (ListInputs, error) {
	if outputFormat != "" && outputFormat != outputFormatJSON {
		return ListInputs{}, fmt.Errorf("--output %q is not supported; only %q is accepted", outputFormat, outputFormatJSON)
	}

	var statuses []workflowdataclient.ExecutionStatus
	if statusFlag != "" {
		s := workflowdataclient.ExecutionStatus(strings.ToUpper(statusFlag))
		if err := validateExecutionStatus(s); err != nil {
			return ListInputs{}, err
		}
		statuses = []workflowdataclient.ExecutionStatus{s}
	}

	var from, to *time.Time
	if startFlag != "" {
		t, err := time.Parse(time.RFC3339, startFlag)
		if err != nil {
			return ListInputs{}, fmt.Errorf("--start: invalid ISO8601 datetime %q (expected e.g. 2006-01-02T15:04:05Z)", startFlag)
		}
		from = &t
	}
	if endFlag != "" {
		t, err := time.Parse(time.RFC3339, endFlag)
		if err != nil {
			return ListInputs{}, fmt.Errorf("--end: invalid ISO8601 datetime %q (expected e.g. 2006-01-02T15:04:05Z)", endFlag)
		}
		to = &t
	}

	return ListInputs{
		Statuses:     statuses,
		From:         from,
		To:           to,
		Limit:        limit,
		OutputFormat: outputFormat,
	}, nil
}

// resolveWorkflowUUID accepts either a UUID (detected by length/format) or a
// workflow name, and returns the platform UUID to use as a filter.
func (h *ListHandler) resolveWorkflowUUID(ctx context.Context, arg string) (string, error) {
	if looksLikeUUID(arg) {
		return arg, nil
	}

	// Treat arg as a workflow name — look it up via the platform API.
	spinner := ui.NewSpinner()
	spinner.Start(fmt.Sprintf("Resolving workflow %q...", arg))
	rows, err := h.wdc.SearchByName(ctx, arg, workflowdataclient.DefaultPageSize)
	spinner.Stop()
	if err != nil {
		return "", fmt.Errorf("resolving workflow name %q: %w", arg, err)
	}

	// Exact-name match only (SearchByName is a contains match on the server).
	var matches []workflowdataclient.Workflow
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Name), arg) {
			matches = append(matches, r)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no workflow found with name %q", arg)
	}

	// Prefer an ACTIVE workflow when multiple versions exist.
	var active []workflowdataclient.Workflow
	for _, r := range matches {
		if strings.EqualFold(r.Status, "ACTIVE") {
			active = append(active, r)
		}
	}
	if len(active) == 1 {
		return active[0].WorkflowID, nil
	}
	if len(active) > 1 {
		return "", fmt.Errorf("multiple ACTIVE workflows named %q found; provide the workflow UUID instead", arg)
	}

	// No ACTIVE found — fall back to first match and warn.
	if !h.nonInteractive {
		ui.Warning(fmt.Sprintf("No ACTIVE deployment for workflow %q; showing executions for the first match (status: %s)", arg, matches[0].Status))
	}
	if matches[0].WorkflowID == "" {
		return "", fmt.Errorf("workflow %q resolved but has no UUID; try providing the UUID directly", arg)
	}
	return matches[0].WorkflowID, nil
}

// ExecuteWithArg resolves workflowArg (UUID or name) then calls Execute.
// It is the entry point used when a positional argument is provided.
func (h *ListHandler) ExecuteWithArg(ctx context.Context, workflowArg string, in ListInputs) error {
	uuid, err := h.resolveWorkflowUUID(ctx, workflowArg)
	if err != nil {
		return err
	}
	in.WorkflowUUID = &uuid
	return h.Execute(ctx, in)
}

// Execute fetches and renders executions.
func (h *ListHandler) Execute(ctx context.Context, in ListInputs) error {
	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching executions...")
	rows, err := h.wdc.ListExecutions(ctx, workflowdataclient.ListExecutionsInput{
		WorkflowUUID: in.WorkflowUUID,
		Statuses:     in.Statuses,
		From:         in.From,
		To:           in.To,
		Limit:        in.Limit,
	})
	spinner.Stop()
	if err != nil {
		return err
	}

	if in.OutputFormat == outputFormatJSON {
		return workflowrender.PrintExecutionsJSON(rows)
	}
	workflowrender.PrintExecutionsTable(rows)
	return nil
}

func newList(runtimeContext *runtime.Context) *cobra.Command {
	var statusFlag string
	var startFlag string
	var endFlag string
	var limit int
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "list [workflow-uuid-or-name]",
		Short: "List recent executions for a workflow",
		Long: `List workflow executions from the CRE platform.

The optional argument accepts either a workflow UUID or a workflow name.
When a name is given the CLI resolves it to the active deployment's UUID.
When omitted, executions across all workflows are returned.`,
		Example: "cre workflow execution list\n" +
			"  cre workflow execution list my-workflow\n" +
			"  cre workflow execution list 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g\n" +
			"  cre workflow execution list my-workflow --status FAILURE\n" +
			"  cre workflow execution list my-workflow --start 2026-01-01T00:00:00Z --end 2026-01-02T00:00:00Z\n" +
			"  cre workflow execution list my-workflow --limit 50 --output json",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newListHandler(runtimeContext)
			in, err := h.resolveListInputs(cmd.Context(), statusFlag, startFlag, endFlag, limit, outputFormat)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				return h.ExecuteWithArg(cmd.Context(), args[0], in)
			}
			return h.Execute(cmd.Context(), in)
		},
	}

	cmd.Flags().StringVar(&statusFlag, "status", "", "Filter by execution status (TRIGGERED, IN_PROGRESS, SUCCESS, FAILURE)")
	cmd.Flags().StringVar(&startFlag, "start", "", "Start of time range in ISO8601 format (e.g. 2026-01-01T00:00:00Z)")
	cmd.Flags().StringVar(&endFlag, "end", "", "End of time range in ISO8601 format (e.g. 2026-01-02T00:00:00Z)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of executions to return (max 100)")
	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints a JSON array to stdout`)

	return cmd
}

// looksLikeUUID returns true when s has the standard UUID shape (8-4-4-4-12).
func looksLikeUUID(s string) bool {
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return false
	}
	lengths := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != lengths[i] {
			return false
		}
	}
	return true
}

// validateExecutionStatus returns an error when s is not a known platform status.
func validateExecutionStatus(s workflowdataclient.ExecutionStatus) error {
	for _, v := range workflowdataclient.ValidExecutionStatuses {
		if s == v {
			return nil
		}
	}
	valid := make([]string, len(workflowdataclient.ValidExecutionStatuses))
	for i, v := range workflowdataclient.ValidExecutionStatuses {
		valid[i] = string(v)
	}
	return fmt.Errorf("--status %q is not valid; accepted values: %s", s, strings.Join(valid, ", "))
}

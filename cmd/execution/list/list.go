package list

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
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

// Inputs holds resolved and validated flag/arg values for execution list.
type Inputs struct {
	// WorkflowRef is an optional workflow name or on-chain WorkflowId from the positional arg.
	WorkflowRef string
	Statuses    []workflowdataclient.ExecutionStatus
	From        *time.Time
	To          *time.Time
	Limit       int
	OutputFormat string
	NonInteractive bool
}

func resolveInputs(
	workflowRef string,
	statusFlag, startFlag, endFlag string,
	limit int,
	outputFormat string,
	jsonFlag bool,
	nonInteractive bool,
) (Inputs, error) {
	outputFormat, err := workflowresolve.ResolveOutputFormat(outputFormat, jsonFlag)
	if err != nil {
		return Inputs{}, err
	}

	var statuses []workflowdataclient.ExecutionStatus
	if statusFlag != "" {
		s := workflowdataclient.ExecutionStatus(strings.ToUpper(statusFlag))
		if err := validateExecutionStatus(s); err != nil {
			return Inputs{}, err
		}
		statuses = []workflowdataclient.ExecutionStatus{s}
	}

	var from, to *time.Time
	if startFlag != "" {
		t, err := time.Parse(time.RFC3339, startFlag)
		if err != nil {
			return Inputs{}, fmt.Errorf("--start: invalid ISO8601 datetime %q (expected e.g. 2006-01-02T15:04:05Z)", startFlag)
		}
		from = &t
	}
	if endFlag != "" {
		t, err := time.Parse(time.RFC3339, endFlag)
		if err != nil {
			return Inputs{}, fmt.Errorf("--end: invalid ISO8601 datetime %q (expected e.g. 2006-01-02T15:04:05Z)", endFlag)
		}
		to = &t
	}

	return Inputs{
		WorkflowRef:    workflowRef,
		Statuses:       statuses,
		From:           from,
		To:             to,
		Limit:          limit,
		OutputFormat:   outputFormat,
		NonInteractive: nonInteractive,
	}, nil
}

// Handler fetches and renders workflow executions.
type Handler struct {
	credentials *credentials.Credentials
	wdc         *workflowdataclient.Client
}

// NewHandler builds a Handler with a real WorkflowDataClient.
func NewHandler(ctx *runtime.Context) *Handler {
	gql := graphqlclient.New(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
	wdc := workflowdataclient.New(gql, ctx.Logger)
	return &Handler{credentials: ctx.Credentials, wdc: wdc}
}

// NewHandlerWithClient builds a Handler with a pre-built client (for testing).
func NewHandlerWithClient(ctx *runtime.Context, wdc *workflowdataclient.Client) *Handler {
	return &Handler{credentials: ctx.Credentials, wdc: wdc}
}

// Execute lists executions applying filters from inputs.
func (h *Handler) Execute(ctx context.Context, inputs Inputs) error {
	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	var workflowUUID *string
	if inputs.WorkflowRef != "" {
		uuid, err := workflowresolve.ResolveWorkflowUUID(ctx, h.wdc, inputs.WorkflowRef, workflowresolve.ResolveOptions{
			NonInteractive: inputs.NonInteractive,
		})
		if err != nil {
			return err
		}
		workflowUUID = &uuid
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching executions...")
	rows, err := h.wdc.ListExecutions(ctx, workflowdataclient.ListExecutionsInput{
		WorkflowUUID: workflowUUID,
		Statuses:     inputs.Statuses,
		From:         inputs.From,
		To:           inputs.To,
		Limit:        inputs.Limit,
	})
	spinner.Stop()
	if err != nil {
		return err
	}

	if inputs.OutputFormat == workflowresolve.OutputFormatJSON {
		return workflowresolve.PrintExecutionsJSON(rows)
	}
	workflowresolve.PrintExecutionsTable(rows)
	return nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var statusFlag string
	var startFlag string
	var endFlag string
	var limit int
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "list [workflow-id-or-name]",
		Short: "List recent executions for a workflow",
		Long: `List workflow executions from the CRE platform.

The optional argument accepts either an on-chain Workflow ID (64-char hex,
visible in 'cre workflow list') or a workflow name. When omitted, executions
across all workflows are returned.`,
		Example: "cre execution list\n" +
			"  cre execution list my-workflow\n" +
			"  cre execution list 00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856\n" +
			"  cre execution list my-workflow --status FAILURE\n" +
			"  cre execution list my-workflow --start 2026-01-01T00:00:00Z --end 2026-01-02T00:00:00Z\n" +
			"  cre execution list my-workflow --limit 50 --output json",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workflowRef := ""
			if len(args) == 1 {
				workflowRef = args[0]
			}
			nonInteractive := false
			if runtimeContext.Viper != nil {
				nonInteractive = runtimeContext.Viper.GetBool(settings.Flags.NonInteractive.Name)
			}
			inputs, err := resolveInputs(workflowRef, statusFlag, startFlag, endFlag, limit, outputFormat, jsonFlag, nonInteractive)
			if err != nil {
				return err
			}
			return NewHandler(runtimeContext).Execute(cmd.Context(), inputs)
		},
	}

	cmd.Flags().StringVar(&statusFlag, "status", "", "Filter by execution status (TRIGGERED, IN_PROGRESS, SUCCESS, FAILURE)")
	cmd.Flags().StringVar(&startFlag, "start", "", "Start of time range in ISO8601 format (e.g. 2026-01-01T00:00:00Z)")
	cmd.Flags().StringVar(&endFlag, "end", "", "End of time range in ISO8601 format (e.g. 2026-01-02T00:00:00Z)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of executions to return (max 100)")
	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints a JSON array to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")

	return cmd
}

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

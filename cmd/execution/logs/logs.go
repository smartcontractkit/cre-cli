package logs

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

// Inputs holds resolved and validated flag/arg values for execution logs.
type Inputs struct {
	ExecutionRef string
	// NodeFilter is applied client-side; the API has no server-side node filter.
	NodeFilter   string
	OutputFormat string
}

func resolveInputs(executionRef, nodeFilter, outputFormat string, jsonFlag bool) (Inputs, error) {
	outputFormat, err := workflowresolve.ResolveOutputFormat(outputFormat, jsonFlag)
	if err != nil {
		return Inputs{}, err
	}
	return Inputs{
		ExecutionRef: executionRef,
		NodeFilter:   nodeFilter,
		OutputFormat: outputFormat,
	}, nil
}

// Handler fetches and renders execution logs.
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

// Execute fetches and renders execution logs.
func (h *Handler) Execute(ctx context.Context, inputs Inputs) error {
	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	uuid, err := workflowresolve.ResolveExecutionUUID(ctx, h.wdc, inputs.ExecutionRef)
	if err != nil {
		return err
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching execution logs...")
	logs, err := h.wdc.ListExecutionLogs(ctx, uuid)
	spinner.Stop()
	if err != nil {
		return err
	}

	if inputs.OutputFormat == workflowresolve.OutputFormatJSON {
		return workflowresolve.PrintLogsJSON(logs, inputs.NodeFilter)
	}
	workflowresolve.PrintLogsTable(logs, inputs.NodeFilter)
	return nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var nodeFilter string
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "logs <execution-uuid>",
		Short: "Show logs emitted during a workflow execution",
		Long: `Fetch and display all log lines emitted during a workflow execution.
Use --node to filter to a specific capability node (client-side filter).`,
		Example: "cre execution logs 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g\n" +
			"  cre execution logs 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --node ProcessData\n" +
			"  cre execution logs 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --output json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := resolveInputs(args[0], nodeFilter, outputFormat, jsonFlag)
			if err != nil {
				return err
			}
			return NewHandler(runtimeContext).Execute(cmd.Context(), inputs)
		},
	}

	cmd.Flags().StringVar(&nodeFilter, "node", "", "Filter logs to a specific node/capability ID (case-insensitive)")
	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints a JSON array to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")
	return cmd
}

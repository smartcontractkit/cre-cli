package execution

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/workflowrender"
)

type LogsInputs struct {
	ExecutionUUID string
	// NodeFilter is applied client-side; the API has no server-side node filter.
	NodeFilter   string
	OutputFormat string
}

type LogsHandler struct {
	credentials *credentials.Credentials
	wdc         *workflowdataclient.Client
}

func NewLogsHandler(ctx *runtime.Context) *LogsHandler {
	gql := graphqlclient.New(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
	wdc := workflowdataclient.New(gql, ctx.Logger)
	return &LogsHandler{credentials: ctx.Credentials, wdc: wdc}
}

func NewLogsHandlerWithClient(ctx *runtime.Context, wdc *workflowdataclient.Client) *LogsHandler {
	return &LogsHandler{credentials: ctx.Credentials, wdc: wdc}
}

func resolveLogsInputs(executionUUID, nodeFilter, outputFormat string, jsonFlag bool) (LogsInputs, error) {
	if jsonFlag {
		outputFormat = outputFormatJSON
	}
	if outputFormat != "" && outputFormat != outputFormatJSON {
		return LogsInputs{}, fmt.Errorf("--output %q is not supported; only %q is accepted", outputFormat, outputFormatJSON)
	}
	return LogsInputs{
		ExecutionUUID: executionUUID,
		NodeFilter:    nodeFilter,
		OutputFormat:  outputFormat,
	}, nil
}

func (h *LogsHandler) Execute(ctx context.Context, in LogsInputs) error {
	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching execution logs...")
	logs, err := h.wdc.ListExecutionLogs(ctx, in.ExecutionUUID)
	spinner.Stop()
	if err != nil {
		return err
	}

	if in.OutputFormat == outputFormatJSON {
		return workflowrender.PrintLogsJSON(logs, in.NodeFilter)
	}
	workflowrender.PrintLogsTable(logs, in.NodeFilter)
	return nil
}

func newLogs(runtimeContext *runtime.Context) *cobra.Command {
	var nodeFilter string
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "logs <execution-uuid>",
		Short: "Show logs emitted during a workflow execution",
		Long: `Fetch and display all log lines emitted during a workflow execution.
Use --node to filter to a specific capability node (client-side filter).`,
		Example: "cre workflow execution logs 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g\n" +
			"  cre workflow execution logs 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --node ProcessData\n" +
			"  cre workflow execution logs 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --output json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := resolveLogsInputs(args[0], nodeFilter, outputFormat, jsonFlag)
			if err != nil {
				return err
			}
			return NewLogsHandler(runtimeContext).Execute(cmd.Context(), in)
		},
	}

	cmd.Flags().StringVar(&nodeFilter, "node", "", "Filter logs to a specific node/capability ID (case-insensitive)")
	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints a JSON array to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")
	return cmd
}

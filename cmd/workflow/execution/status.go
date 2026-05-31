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

type StatusInputs struct {
	ExecutionUUID string
	OutputFormat  string
}

type StatusHandler struct {
	credentials *credentials.Credentials
	wdc         *workflowdataclient.Client
}

func NewStatusHandler(ctx *runtime.Context) *StatusHandler {
	gql := graphqlclient.New(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
	wdc := workflowdataclient.New(gql, ctx.Logger)
	return &StatusHandler{credentials: ctx.Credentials, wdc: wdc}
}

func NewStatusHandlerWithClient(ctx *runtime.Context, wdc *workflowdataclient.Client) *StatusHandler {
	return &StatusHandler{credentials: ctx.Credentials, wdc: wdc}
}

func resolveStatusInputs(executionUUID, outputFormat string) (StatusInputs, error) {
	if outputFormat != "" && outputFormat != outputFormatJSON {
		return StatusInputs{}, fmt.Errorf("--output %q is not supported; only %q is accepted", outputFormat, outputFormatJSON)
	}
	return StatusInputs{ExecutionUUID: executionUUID, OutputFormat: outputFormat}, nil
}

func (h *StatusHandler) Execute(ctx context.Context, in StatusInputs) error {
	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching execution...")
	exec, err := h.wdc.GetExecution(ctx, in.ExecutionUUID)
	spinner.Stop()
	if err != nil {
		return err
	}

	if in.OutputFormat == outputFormatJSON {
		return workflowrender.PrintExecutionDetailJSON(*exec)
	}
	workflowrender.PrintExecutionDetailTable(*exec)
	return nil
}

func newStatus(runtimeContext *runtime.Context) *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "status <execution-uuid>",
		Short: "Show detailed status of a single execution",
		Long: `Fetch and display the full status of a workflow execution, including
top-level errors when the execution has failed.`,
		Example: "cre workflow execution status 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g\n" +
			"  cre workflow execution status 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --output json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := resolveStatusInputs(args[0], outputFormat)
			if err != nil {
				return err
			}
			return NewStatusHandler(runtimeContext).Execute(cmd.Context(), in)
		},
	}

	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints JSON to stdout`)
	return cmd
}

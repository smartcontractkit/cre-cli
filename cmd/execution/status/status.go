package status

import (
	"context"
	"fmt"
	"sync"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

// Inputs holds resolved and validated flag/arg values for execution status.
type Inputs struct {
	ExecutionRef string
	OutputFormat string
}

func resolveInputs(executionRef, outputFormat string, jsonFlag bool) (Inputs, error) {
	outputFormat, err := workflowresolve.ResolveOutputFormat(outputFormat, jsonFlag)
	if err != nil {
		return Inputs{}, err
	}
	return Inputs{ExecutionRef: executionRef, OutputFormat: outputFormat}, nil
}

// Handler fetches and renders a single execution detail view.
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

// Execute fetches and renders execution status.
func (h *Handler) Execute(ctx context.Context, inputs Inputs) error {
	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching execution...")

	uuid, err := workflowresolve.ResolveExecutionUUID(ctx, h.wdc, inputs.ExecutionRef)
	if err != nil {
		spinner.Stop()
		return err
	}

	var (
		exec       *workflowdataclient.Execution
		failEvents []workflowdataclient.ExecutionEvent
		execErr    error
		wg         sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		exec, execErr = h.wdc.GetExecution(ctx, uuid)
	}()
	wg.Wait()

	if execErr == nil && exec.Status == workflowdataclient.ExecutionStatusFailure {
		failEvents, err = h.wdc.ListExecutionEvents(ctx, workflowdataclient.ListEventsInput{
			ExecutionUUID: uuid,
			Status:        new(string(workflowdataclient.ExecutionStatusFailure)),
		})
		if err != nil {
			return err
		}
	}

	spinner.Stop()
	if execErr != nil {
		return execErr
	}

	if inputs.OutputFormat == workflowresolve.OutputFormatJSON {
		return workflowresolve.PrintExecutionDetailJSON(*exec, failEvents)
	}
	workflowresolve.PrintExecutionDetailTable(*exec, failEvents)
	return nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "status <execution-uuid>",
		Short: "Show detailed status of a single execution",
		Long: `Fetch and display the full status of a workflow execution, including
top-level errors when the execution has failed.`,
		Example: "cre execution status 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g\n" +
			"  cre execution status 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --output json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := resolveInputs(args[0], outputFormat, jsonFlag)
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

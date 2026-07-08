package events

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

// Inputs holds resolved and validated flag/arg values for execution events.
type Inputs struct {
	ExecutionRef string
	CapabilityID *string
	Status       *string
	OutputFormat string
}

func resolveInputs(executionRef, capabilityID, status, outputFormat string, jsonFlag bool) (Inputs, error) {
	outputFormat, err := workflowresolve.ResolveOutputFormat(outputFormat, jsonFlag)
	if err != nil {
		return Inputs{}, err
	}
	in := Inputs{
		ExecutionRef: executionRef,
		OutputFormat: outputFormat,
	}
	if capabilityID != "" {
		in.CapabilityID = &capabilityID
	}
	if status != "" {
		in.Status = &status
	}
	return in, nil
}

// Handler fetches and renders execution capability events.
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

// Execute fetches and renders execution events.
func (h *Handler) Execute(ctx context.Context, inputs Inputs) error {
	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching execution events...")

	uuid, err := workflowresolve.ResolveExecutionUUID(ctx, h.wdc, inputs.ExecutionRef)
	if err != nil {
		spinner.Stop()
		return err
	}

	events, err := h.wdc.ListExecutionEvents(ctx, workflowdataclient.ListEventsInput{
		ExecutionUUID: uuid,
		CapabilityID:  inputs.CapabilityID,
		Status:        inputs.Status,
	})
	spinner.Stop()
	if err != nil {
		return err
	}

	if inputs.OutputFormat == workflowresolve.OutputFormatJSON {
		return workflowresolve.PrintEventsJSON(events)
	}
	workflowresolve.PrintEventsTable(events)
	return nil
}

// New returns the cobra command.
func New(runtimeContext *runtime.Context) *cobra.Command {
	var capabilityID string
	var status string
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "events <execution-uuid>",
		Short: "Show the node/capability event timeline for an execution",
		Long: `Fetch and display the ordered sequence of capability events for a workflow
execution, including per-event status, method, duration, and any errors.`,
		Example: "cre execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g\n" +
			"  cre execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --capability fetch-price\n" +
			"  cre execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --status FAILURE\n" +
			"  cre execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --output json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := resolveInputs(args[0], capabilityID, status, outputFormat, jsonFlag)
			if err != nil {
				return err
			}
			return NewHandler(runtimeContext).Execute(cmd.Context(), inputs)
		},
	}

	cmd.Flags().StringVar(&capabilityID, "capability", "", "Filter events to a specific capability ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter events by status (e.g. FAILURE)")
	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints a JSON array to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")
	return cmd
}

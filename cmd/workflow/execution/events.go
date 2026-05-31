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

type EventsInputs struct {
	ExecutionUUID string
	CapabilityID  *string
	Status        *string
	OutputFormat  string
}

type EventsHandler struct {
	credentials *credentials.Credentials
	wdc         *workflowdataclient.Client
}

func NewEventsHandler(ctx *runtime.Context) *EventsHandler {
	gql := graphqlclient.New(ctx.Credentials, ctx.EnvironmentSet, ctx.Logger)
	wdc := workflowdataclient.New(gql, ctx.Logger)
	return &EventsHandler{credentials: ctx.Credentials, wdc: wdc}
}

func NewEventsHandlerWithClient(ctx *runtime.Context, wdc *workflowdataclient.Client) *EventsHandler {
	return &EventsHandler{credentials: ctx.Credentials, wdc: wdc}
}

func resolveEventsInputs(executionUUID, capabilityID, status, outputFormat string, jsonFlag bool) (EventsInputs, error) {
	if jsonFlag {
		outputFormat = outputFormatJSON
	}
	if outputFormat != "" && outputFormat != outputFormatJSON {
		return EventsInputs{}, fmt.Errorf("--output %q is not supported; only %q is accepted", outputFormat, outputFormatJSON)
	}
	in := EventsInputs{
		ExecutionUUID: executionUUID,
		OutputFormat:  outputFormat,
	}
	if capabilityID != "" {
		in.CapabilityID = &capabilityID
	}
	if status != "" {
		in.Status = &status
	}
	return in, nil
}

func (h *EventsHandler) Execute(ctx context.Context, in EventsInputs) error {
	if h.credentials == nil {
		return fmt.Errorf("credentials not available — run `cre login` and retry")
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching execution events...")
	events, err := h.wdc.ListExecutionEvents(ctx, workflowdataclient.ListEventsInput{
		ExecutionUUID: in.ExecutionUUID,
		CapabilityID:  in.CapabilityID,
		Status:        in.Status,
	})
	spinner.Stop()
	if err != nil {
		return err
	}

	if in.OutputFormat == outputFormatJSON {
		return workflowrender.PrintEventsJSON(events)
	}
	workflowrender.PrintEventsTable(events)
	return nil
}

func newEvents(runtimeContext *runtime.Context) *cobra.Command {
	var capabilityID string
	var status string
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:   "events <execution-uuid>",
		Short: "Show the node/capability event timeline for an execution",
		Long: `Fetch and display the ordered sequence of capability events for a workflow
execution, including per-event status, method, duration, and any errors.`,
		Example: "cre workflow execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g\n" +
			"  cre workflow execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --capability fetch-price\n" +
			"  cre workflow execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --status FAILURE\n" +
			"  cre workflow execution events 7f3d8a12-b1c2-4d3e-9f0a-1b2c3d4e5f6g --output json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := resolveEventsInputs(args[0], capabilityID, status, outputFormat, jsonFlag)
			if err != nil {
				return err
			}
			return NewEventsHandler(runtimeContext).Execute(cmd.Context(), in)
		},
	}

	cmd.Flags().StringVar(&capabilityID, "capability", "", "Filter events to a specific capability ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter events by status (e.g. FAILURE)")
	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints a JSON array to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")
	return cmd
}

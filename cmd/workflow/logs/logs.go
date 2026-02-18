package logs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

const pollInterval = 5 * time.Second

func New(runtimeContext *runtime.Context) *cobra.Command {
	var follow bool
	var limit int

	logsCmd := &cobra.Command{
		Use:   "logs <workflow-name>",
		Short: "Show execution history for a workflow",
		Long:  "Fetches and displays recent execution history for the specified workflow from the CRE platform.",
		Args:  cobra.ExactArgs(1),
		Example: `  cre workflow logs my-workflow
  cre workflow logs my-workflow --follow
  cre workflow logs my-workflow --limit 5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(runtimeContext, args[0], follow, limit)
			return h.Execute(cmd.Context())
		},
	}

	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Keep polling for new executions")
	logsCmd.Flags().IntVarP(&limit, "limit", "n", 10, "Number of recent executions to show")

	return logsCmd
}

type handler struct {
	log            *zerolog.Logger
	credentials    *credentials.Credentials
	environmentSet *environments.EnvironmentSet
	workflowName   string
	follow         bool
	limit          int
}

func newHandler(ctx *runtime.Context, workflowName string, follow bool, limit int) *handler {
	return &handler{
		log:            ctx.Logger,
		credentials:    ctx.Credentials,
		environmentSet: ctx.EnvironmentSet,
		workflowName:   workflowName,
		follow:         follow,
		limit:          limit,
	}
}

// GraphQL response types

type workflowsResponse struct {
	Workflows struct {
		Data  []workflowEntry `json:"data"`
		Count int             `json:"count"`
	} `json:"workflows"`
}

type workflowEntry struct {
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type executionsResponse struct {
	WorkflowExecutions struct {
		Data  []execution `json:"data"`
		Count int         `json:"count"`
	} `json:"workflowExecutions"`
}

type execution struct {
	UUID       string     `json:"uuid"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

type eventsResponse struct {
	WorkflowExecutionEvents struct {
		Data []executionEvent `json:"data"`
	} `json:"workflowExecutionEvents"`
}

type executionEvent struct {
	CapabilityID string     `json:"capabilityID"`
	Status       string     `json:"status"`
	Errors       []capError `json:"errors"`
}

type capError struct {
	Error string `json:"error"`
	Count int    `json:"count"`
}

func (h *handler) Execute(ctx context.Context) error {
	client := graphqlclient.New(h.credentials, h.environmentSet, h.log)

	workflowUUID, err := h.findWorkflow(ctx, client)
	if err != nil {
		return err
	}

	fmt.Printf("\nWorkflow: %s\n\n", h.workflowName)

	executions, err := h.fetchExecutions(ctx, client, workflowUUID)
	if err != nil {
		return err
	}

	headerPrinted := false
	if len(executions) == 0 && !h.follow {
		fmt.Println("No executions found.")
		return nil
	}

	if len(executions) > 0 {
		printHeader()
		headerPrinted = true
		h.printExecutions(ctx, client, executions)
	}

	if !h.follow {
		return nil
	}

	if !headerPrinted {
		fmt.Println("Waiting for executions...")
	}

	lastSeenUUID := ""
	if len(executions) > 0 {
		lastSeenUUID = executions[0].UUID
	}

	for {
		select {
		case <-time.After(pollInterval):
		case <-ctx.Done():
			return nil
		}

		executions, err = h.fetchExecutions(ctx, client, workflowUUID)
		if err != nil {
			h.log.Error().Err(err).Msg("failed to fetch executions, retrying")
			continue
		}

		newExecs := filterNew(executions, lastSeenUUID)
		if len(newExecs) > 0 {
			if !headerPrinted {
				printHeader()
				headerPrinted = true
			}
			h.printExecutions(ctx, client, newExecs)
			lastSeenUUID = executions[0].UUID
		}
	}
}

func (h *handler) findWorkflow(ctx context.Context, client *graphqlclient.Client) (string, error) {
	req := graphql.NewRequest(`query FindWorkflow($input: WorkflowsInput!) {
		workflows(input: $input) {
			data { uuid name status }
			count
		}
	}`)
	req.Var("input", map[string]any{
		"search": h.workflowName,
		"page":   map[string]int{"number": 0, "size": 20},
	})

	var resp workflowsResponse
	if err := client.Execute(ctx, req, &resp); err != nil {
		return "", fmt.Errorf("failed to search for workflow: %w", err)
	}

	for _, w := range resp.Workflows.Data {
		if w.Name == h.workflowName {
			return w.UUID, nil
		}
	}

	if len(resp.Workflows.Data) == 0 {
		return "", fmt.Errorf("no workflow found matching %q", h.workflowName)
	}

	names := make([]string, len(resp.Workflows.Data))
	for i, w := range resp.Workflows.Data {
		names[i] = w.Name
	}
	return "", fmt.Errorf("no exact match for %q; found: %s", h.workflowName, strings.Join(names, ", "))
}

func (h *handler) fetchExecutions(ctx context.Context, client *graphqlclient.Client, workflowUUID string) ([]execution, error) {
	req := graphql.NewRequest(`query GetExecutions($input: WorkflowExecutionsInput!) {
		workflowExecutions(input: $input) {
			data { uuid status startedAt finishedAt }
			count
		}
	}`)
	req.Var("input", map[string]any{
		"workflowUuid": workflowUUID,
		"orderBy":      map[string]string{"field": "STARTED_AT", "order": "DESC"},
		"page":         map[string]int{"number": 0, "size": h.limit},
	})

	var resp executionsResponse
	if err := client.Execute(ctx, req, &resp); err != nil {
		return nil, fmt.Errorf("failed to fetch executions: %w", err)
	}

	return resp.WorkflowExecutions.Data, nil
}

// filterNew returns executions that are newer than lastSeenUUID.
// Executions are expected in DESC order (newest first).
func filterNew(executions []execution, lastSeenUUID string) []execution {
	if lastSeenUUID == "" {
		return executions
	}
	for i, e := range executions {
		if e.UUID == lastSeenUUID {
			return executions[:i]
		}
	}
	// lastSeenUUID not found in current page, all are new
	return executions
}

func printHeader() {
	fmt.Printf("%-24s  %-12s  %-10s  %s\n", "TIMESTAMP", "STATUS", "DURATION", "EXECUTION ID")
}

func (h *handler) printExecutions(ctx context.Context, client *graphqlclient.Client, executions []execution) {
	// Print oldest first (executions are in DESC order)
	for i := len(executions) - 1; i >= 0; i-- {
		e := executions[i]
		duration := "running"
		if e.FinishedAt != nil {
			duration = formatDuration(e.FinishedAt.Sub(e.StartedAt))
		}

		fmt.Printf("%-24s  %-12s  %-10s  %s\n",
			e.StartedAt.Format(time.RFC3339),
			strings.ToLower(e.Status),
			duration,
			shortUUID(e.UUID),
		)

		if e.Status == "FAILURE" {
			h.printErrors(ctx, client, e.UUID)
		}
	}
}

func (h *handler) printErrors(ctx context.Context, client *graphqlclient.Client, executionUUID string) {
	req := graphql.NewRequest(`query GetEvents($input: WorkflowExecutionEventsInput!) {
		workflowExecutionEvents(input: $input) {
			data { capabilityID status errors { error count } }
		}
	}`)
	req.Var("input", map[string]any{
		"workflowExecutionUUID": executionUUID,
	})

	var resp eventsResponse
	if err := client.Execute(ctx, req, &resp); err != nil {
		h.log.Debug().Err(err).Msg("failed to fetch execution events")
		return
	}

	for _, ev := range resp.WorkflowExecutionEvents.Data {
		if ev.Status == "failure" && len(ev.Errors) > 0 {
			errMsg := ev.Errors[0].Error
			if len(errMsg) > 120 {
				tail := errMsg[len(errMsg)-len(errMsg)*2/5:] // last 40%
				head := 120 - len(tail) - 3
				if head < 0 {
					head = 0
				}
				errMsg = errMsg[:head] + "..." + tail
			}
			fmt.Printf("  -> %s: %s\n", ev.CapabilityID, errMsg)
		}
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func shortUUID(uuid string) string {
	if len(uuid) >= 8 {
		return uuid[:8]
	}
	return uuid
}

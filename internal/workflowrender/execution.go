package workflowrender

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// ---- List executions ----

type executionJSON struct {
	UUID         string  `json:"uuid"`
	WorkflowUUID string  `json:"workflowUUID"`
	WorkflowName string  `json:"workflowName"`
	Status       string  `json:"status"`
	StartedAt    string  `json:"startedAt"`
	FinishedAt   *string `json:"finishedAt,omitempty"`
	DurationSec  *string `json:"duration,omitempty"`
	CreditUsed   *string `json:"creditUsed,omitempty"`
}

func toExecutionJSON(e workflowdataclient.Execution) executionJSON {
	started := e.StartedAt.UTC().Format(time.RFC3339)
	j := executionJSON{
		UUID:         e.UUID,
		WorkflowUUID: e.WorkflowUUID,
		WorkflowName: e.WorkflowName,
		Status:       string(e.Status),
		StartedAt:    started,
		CreditUsed:   e.CreditUsed,
	}
	if e.FinishedAt != nil {
		f := e.FinishedAt.UTC().Format(time.RFC3339)
		j.FinishedAt = &f
		d := formatDuration(e.FinishedAt.Sub(e.StartedAt))
		j.DurationSec = &d
	}
	return j
}

// PrintExecutionsJSON marshals a slice of executions as an indented JSON array to stdout.
func PrintExecutionsJSON(rows []workflowdataclient.Execution) error {
	out := make([]executionJSON, 0, len(rows))
	for _, e := range rows {
		out = append(out, toExecutionJSON(e))
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// PrintExecutionsTable renders executions as a bulleted list to stdout.
func PrintExecutionsTable(rows []workflowdataclient.Execution) {
	ui.Line()
	if len(rows) == 0 {
		ui.Warning("No executions found")
		ui.Line()
		return
	}

	ui.Bold("Executions")
	ui.Line()

	for i, e := range rows {
		ui.Bold(fmt.Sprintf("%d. %s", i+1, e.UUID))
		ui.Dim(fmt.Sprintf("   Workflow:  %s (%s)", e.WorkflowName, e.WorkflowUUID))
		ui.Dim(fmt.Sprintf("   Status:    %s", e.Status))
		ui.Dim(fmt.Sprintf("   Started:   %s", e.StartedAt.UTC().Format("2006-01-02 15:04:05")))
		if e.FinishedAt != nil {
			ui.Dim(fmt.Sprintf("   Finished:  %s (%s)", e.FinishedAt.UTC().Format("2006-01-02 15:04:05"), formatDuration(e.FinishedAt.Sub(e.StartedAt))))
		}
			ui.Line()
	}
}

// ---- Single execution detail (status command) ----

type executionDetailJSON struct {
	executionJSON
	Errors []executionErrorJSON `json:"errors,omitempty"`
}

type executionErrorJSON struct {
	Error string `json:"error"`
	Count int    `json:"count"`
}

// PrintExecutionDetailJSON marshals a single execution with its errors to stdout.
func PrintExecutionDetailJSON(e workflowdataclient.Execution) error {
	errs := make([]executionErrorJSON, 0, len(e.Errors))
	for _, err := range e.Errors {
		errs = append(errs, executionErrorJSON{Error: err.Error, Count: err.Count})
	}
	detail := executionDetailJSON{
		executionJSON: toExecutionJSON(e),
		Errors:        errs,
	}
	data, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// PrintExecutionDetailTable renders a single execution with its errors as a bulleted detail view.
func PrintExecutionDetailTable(e workflowdataclient.Execution) {
	ui.Line()
	ui.Bold(fmt.Sprintf("Execution: %s", e.UUID))
	ui.Dim(fmt.Sprintf("   Workflow:  %s", e.WorkflowName))
	ui.Dim(fmt.Sprintf("   UUID:      %s", e.WorkflowUUID))
	ui.Dim(fmt.Sprintf("   Status:    %s", e.Status))

	timeStr := e.StartedAt.UTC().Format("2006-01-02 15:04:05")
	if e.FinishedAt != nil {
		timeStr = fmt.Sprintf("%s to %s (%s)",
			e.StartedAt.UTC().Format("2006-01-02 15:04:05"),
			e.FinishedAt.UTC().Format("15:04:05"),
			formatDuration(e.FinishedAt.Sub(e.StartedAt)),
		)
	}
	ui.Dim(fmt.Sprintf("   Time:      %s", timeStr))

	if len(e.Errors) > 0 {
		ui.Line()
		ui.Bold("Top-Level Errors:")
		for _, err := range e.Errors {
			ui.Dim(fmt.Sprintf("   - %s (Count: %d)", err.Error, err.Count))
		}
	}

	ui.Line()
}

// ---- Events ----

type eventJSON struct {
	CapabilityID string                `json:"capabilityID"`
	Status       string                `json:"status"`
	Method       *string               `json:"method,omitempty"`
	StartedAt    string                `json:"startedAt"`
	FinishedAt   *string               `json:"finishedAt,omitempty"`
	Duration     *string               `json:"duration,omitempty"`
	Errors       []capabilityErrorJSON `json:"errors,omitempty"`
}

type capabilityErrorJSON struct {
	Error string `json:"error"`
	Count int    `json:"count"`
}

// PrintEventsJSON marshals events as an indented JSON array to stdout.
func PrintEventsJSON(events []workflowdataclient.ExecutionEvent) error {
	out := make([]eventJSON, 0, len(events))
	for _, ev := range events {
		j := eventJSON{
			CapabilityID: ev.CapabilityID,
			Status:       ev.Status,
			Method:       ev.Method,
			StartedAt:    ev.StartedAt.UTC().Format(time.RFC3339),
		}
		if ev.FinishedAt != nil {
			f := ev.FinishedAt.UTC().Format(time.RFC3339)
			j.FinishedAt = &f
			d := formatDuration(ev.FinishedAt.Sub(ev.StartedAt))
			j.Duration = &d
		}
		for _, e := range ev.Errors {
			j.Errors = append(j.Errors, capabilityErrorJSON{Error: e.Error, Count: e.Count})
		}
		out = append(out, j)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// PrintEventsTable renders events as a bulleted list to stdout.
func PrintEventsTable(events []workflowdataclient.ExecutionEvent) {
	ui.Line()
	if len(events) == 0 {
		ui.Warning("No events found")
		ui.Line()
		return
	}

	ui.Bold("Events")
	ui.Line()

	for i, ev := range events {
		method := "-"
		if ev.Method != nil && *ev.Method != "" {
			method = *ev.Method
		}
		dur := "-"
		if ev.FinishedAt != nil {
			dur = formatDuration(ev.FinishedAt.Sub(ev.StartedAt))
		}

		ui.Bold(fmt.Sprintf("%d. %s", i+1, ev.CapabilityID))
		ui.Dim(fmt.Sprintf("   Method:    %s", method))
		ui.Dim(fmt.Sprintf("   Status:    %s", ev.Status))
		ui.Dim(fmt.Sprintf("   Started:   %s", ev.StartedAt.UTC().Format("2006-01-02 15:04:05")))
		ui.Dim(fmt.Sprintf("   Duration:  %s", dur))
		if len(ev.Errors) > 0 {
			errMsgs := make([]string, 0, len(ev.Errors))
			for _, e := range ev.Errors {
				errMsgs = append(errMsgs, fmt.Sprintf("%s (x%d)", e.Error, e.Count))
			}
			ui.Dim(fmt.Sprintf("   Errors:    %s", strings.Join(errMsgs, "; ")))
		}
		ui.Line()
	}
}

// ---- Logs ----

type logJSON struct {
	NodeID    string `json:"nodeID"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

// PrintLogsJSON marshals logs as an indented JSON array to stdout.
// nodeFilter, if non-empty, restricts output to lines whose NodeID matches (case-insensitive).
func PrintLogsJSON(logs []workflowdataclient.ExecutionLog, nodeFilter string) error {
	out := make([]logJSON, 0, len(logs))
	for _, l := range logs {
		if nodeFilter != "" && !strings.EqualFold(l.NodeID, nodeFilter) {
			continue
		}
		out = append(out, logJSON{
			NodeID:    l.NodeID,
			Timestamp: l.Timestamp.UTC().Format(time.RFC3339),
			Message:   l.Message,
		})
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// PrintLogsTable renders log lines to stdout.
// nodeFilter, if non-empty, restricts output to lines whose NodeID matches (case-insensitive).
func PrintLogsTable(logs []workflowdataclient.ExecutionLog, nodeFilter string) {
	ui.Line()
	printed := 0
	for _, l := range logs {
		if nodeFilter != "" && !strings.EqualFold(l.NodeID, nodeFilter) {
			continue
		}
		ui.Print(fmt.Sprintf("[%s] [%s] %s",
			l.Timestamp.UTC().Format("2006-01-02 15:04:05"),
			l.NodeID,
			l.Message,
		))
		printed++
	}
	if printed == 0 {
		ui.Warning("No logs found")
	}
	ui.Line()
}

// ---- shared helpers ----

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

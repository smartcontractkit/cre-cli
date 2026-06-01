package workflowrender

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// WorkflowStatusView bundles all data for the status command output.
type WorkflowStatusView struct {
	Summary       *workflowdataclient.WorkflowSummary
	Deployment    *workflowdataclient.WorkflowDeploymentRecord
	LastExecution *workflowdataclient.Execution
}

// PrintWorkflowStatusTable renders a rich workflow status view to stdout.
func PrintWorkflowStatusTable(v WorkflowStatusView) {
	s := v.Summary
	ui.Line()
	ui.Bold(fmt.Sprintf("Workflow: %s", s.Name))
	ui.Dim(fmt.Sprintf("   Workflow ID:    %s", s.WorkflowID))
	ui.Dim(fmt.Sprintf("   Status:         %s%s", s.Status, deploymentStatusHint(s.Status)))
	ui.Dim(fmt.Sprintf("   Registered:     %s", s.RegisteredAt.UTC().Format("2006-01-02 15:04:05")))

	if s.ExecutedAt != nil {
		ui.Dim(fmt.Sprintf("   Last executed:  %s", s.ExecutedAt.UTC().Format("2006-01-02 15:04:05")))
		if s.Status == "PENDING" {
			gap := s.ExecutedAt.Sub(s.RegisteredAt)
			ui.Dim(fmt.Sprintf("   Activation gap: %s", formatDuration(gap)))
		}
	} else {
		ui.Dim("   Last executed:  never")
		if s.Status == "PENDING" {
			gap := time.Since(s.RegisteredAt)
			ui.Dim(fmt.Sprintf("   Pending for:    %s", formatDuration(gap)))
		}
	}



	ui.Line()
	ui.Bold("Deployment")
	if v.Deployment != nil {
		d := v.Deployment
		ui.Dim(fmt.Sprintf("   Status:         %s", d.Status))
		ui.Dim(fmt.Sprintf("   Deployed at:    %s", d.DeployedAt.UTC().Format("2006-01-02 15:04:05")))
		if d.TxHash != nil && *d.TxHash != "" {
			ui.Dim(fmt.Sprintf("   Tx hash:        %s", *d.TxHash))
		}
		if d.BinaryURL != nil && *d.BinaryURL != "" {
			ui.Dim(fmt.Sprintf("   Binary URL:     %s", *d.BinaryURL))
		}
		if d.ErrorMessage != nil && *d.ErrorMessage != "" {
			ui.Dim(fmt.Sprintf("   Error:          %s", *d.ErrorMessage))
		}
	} else {
		ui.Dim("   No deployment record found")
	}

	ui.Line()
	ui.Bold("Execution summary")
	ui.Dim(fmt.Sprintf("   Total:          %d", s.ExecutionCount))
	ui.Dim(fmt.Sprintf("   Success:        %d", s.SuccessCount))
	ui.Dim(fmt.Sprintf("   Failure:        %d", s.FailureCount))

	if v.LastExecution != nil {
		e := v.LastExecution
		ui.Line()
		ui.Bold("Last execution")
		ui.Dim(fmt.Sprintf("   ID:             %s", e.UUID))
		ui.Dim(fmt.Sprintf("   Status:         %s", e.Status))
		ui.Dim(fmt.Sprintf("   Started:        %s", e.StartedAt.UTC().Format("2006-01-02 15:04:05")))
		if e.FinishedAt != nil {
			ui.Dim(fmt.Sprintf("   Duration:       %s", formatDuration(e.FinishedAt.Sub(e.StartedAt))))
		}
		if len(e.Errors) > 0 {
			ui.Dim("   Errors:")
			for _, err := range e.Errors {
				ui.Dim(fmt.Sprintf("     - %s (x%d)", err.Error, err.Count))
			}
		}
		ui.Line()
		ui.Bold("Next steps:")
		ui.Dim(fmt.Sprintf("   cre workflow execution list %s", s.Name))
		ui.Dim(fmt.Sprintf("   cre workflow execution events %s", e.UUID))
		ui.Dim(fmt.Sprintf("   cre workflow execution logs   %s", e.UUID))
	} else if s.Status == "PENDING" {
		ui.Line()
		ui.Dim("   Workflow has not executed yet — it may still be activating in the DON.")
	}

	ui.Line()
}

// PrintWorkflowStatusJSON marshals the status view as indented JSON to stdout.
func PrintWorkflowStatusJSON(v WorkflowStatusView) error {
	s := v.Summary
	out := map[string]any{
		"workflow": map[string]any{
			"name":           s.Name,
			"workflowId":     s.WorkflowID,
			"status":         s.Status,
			"registeredAt":   s.RegisteredAt.UTC().Format(time.RFC3339),
			"lastExecutedAt": timeOrNil(s.ExecutedAt),
			"executionCount": s.ExecutionCount,
			"successCount":   s.SuccessCount,
			"failureCount":   s.FailureCount,
			},
	}

	if v.Deployment != nil {
		d := v.Deployment
		dep := map[string]any{
			"status":     d.Status,
			"deployedAt": d.DeployedAt.UTC().Format(time.RFC3339),
			"txHash":     d.TxHash,
			"binaryURL":  d.BinaryURL,
		}
		if d.ErrorMessage != nil && *d.ErrorMessage != "" {
			dep["errorMessage"] = *d.ErrorMessage
		}
		out["deployment"] = dep
	}

	if v.LastExecution != nil {
		e := v.LastExecution
		errs := make([]map[string]any, 0, len(e.Errors))
		for _, err := range e.Errors {
			errs = append(errs, map[string]any{"error": err.Error, "count": err.Count})
		}
		out["lastExecution"] = map[string]any{
			"uuid":       e.UUID,
			"status":     string(e.Status),
			"startedAt":  e.StartedAt.UTC().Format(time.RFC3339),
			"finishedAt": timeOrNil(e.FinishedAt),
			"errors":     errs,
		}
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// deploymentStatusHint returns an inline warning for non-healthy states.
func deploymentStatusHint(status string) string {
	switch strings.ToUpper(status) {
	case "PENDING":
		return "  ⚠ not yet active in the DON"
	case "FAILED":
		return "  ✗ activation failed"
	case "PAUSED":
		return "  — paused"
	default:
		return ""
	}
}

func timeOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

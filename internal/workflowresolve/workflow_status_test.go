package workflowresolve

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func sampleWorkflowStatusView() WorkflowStatusView {
	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)
	finished := time.Date(2026, 5, 29, 14, 0, 17, 0, time.UTC)
	registered := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	return WorkflowStatusView{
		Summary: &workflowdataclient.WorkflowSummary{
			Name:         "my-workflow",
			WorkflowID:   "abc123onchain",
			OwnerAddress: "0xowner",
			Status:       "ACTIVE",
			RegisteredAt: registered,
		},
		LastExecution: &workflowdataclient.Execution{
			UUID:         "exec-uuid-1",
			ID:           "05ace5cf85ae448b9f42270d42974d35",
			WorkflowName: "my-workflow",
			Status:       workflowdataclient.ExecutionStatusSuccess,
			StartedAt:    started,
			FinishedAt:   &finished,
		},
	}
}

func TestPrintWorkflowStatusTable_ShowsDebugHints(t *testing.T) {
	t.Parallel()
	out := captureStdout(t, func() {
		PrintWorkflowStatusTable(sampleWorkflowStatusView())
	})
	assert.Contains(t, out, "Debug further:")
	assert.Contains(t, out, "cre execution status 05ace5cf85ae448b9f42270d42974d35")
	assert.Contains(t, out, "cre execution events 05ace5cf85ae448b9f42270d42974d35")
	assert.Contains(t, out, "cre execution logs   05ace5cf85ae448b9f42270d42974d35")
}

func TestPrintWorkflowStatusJSON_OmitsDebugHints(t *testing.T) {
	t.Parallel()
	out := captureStdout(t, func() {
		require.NoError(t, PrintWorkflowStatusJSON(sampleWorkflowStatusView()))
	})
	assert.NotContains(t, out, "Debug further:")
	assert.NotContains(t, out, "cre execution")
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &result))
}

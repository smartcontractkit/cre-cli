package workflowresolve

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
)

func sampleExecution() workflowdataclient.Execution {
	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)
	finished := time.Date(2026, 5, 29, 14, 0, 17, 0, time.UTC)
	return workflowdataclient.Execution{
		UUID:         "exec-uuid-1",
		ID:           "05ace5cf85ae448b9f42270d42974d35",
		WorkflowName: "my-workflow",
		WorkflowID:   "abc123onchain",
		Status:       workflowdataclient.ExecutionStatusFailure,
		StartedAt:    started,
		FinishedAt:   &finished,
	}
}

func TestPrintExecutionDetailTable_ShowsDebugHints(t *testing.T) {
	t.Parallel()
	exec := sampleExecution()
	out := captureStdout(t, func() {
		PrintExecutionDetailTable(exec, nil)
	})
	assert.Contains(t, out, "Debug further:")
	assert.Contains(t, out, "cre execution events 05ace5cf85ae448b9f42270d42974d35")
	assert.Contains(t, out, "cre execution logs   05ace5cf85ae448b9f42270d42974d35")
}

func TestPrintExecutionDetailJSON_OmitsDebugHints(t *testing.T) {
	t.Parallel()
	exec := sampleExecution()
	out := captureStdout(t, func() {
		require.NoError(t, PrintExecutionDetailJSON(exec, nil))
	})
	assert.NotContains(t, out, "Debug further:")
	assert.NotContains(t, out, "cre execution")
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &result))
}

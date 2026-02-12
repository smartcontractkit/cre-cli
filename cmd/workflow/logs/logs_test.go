package logs

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

func TestFilterNew(t *testing.T) {
	t.Parallel()

	execs := []execution{
		{UUID: "aaa"},
		{UUID: "bbb"},
		{UUID: "ccc"},
	}

	tests := []struct {
		name         string
		executions   []execution
		lastSeenUUID string
		wantUUIDs    []string
	}{
		{
			name:         "no last seen returns all",
			executions:   execs,
			lastSeenUUID: "",
			wantUUIDs:    []string{"aaa", "bbb", "ccc"},
		},
		{
			name:         "last seen is newest returns empty",
			executions:   execs,
			lastSeenUUID: "aaa",
			wantUUIDs:    nil,
		},
		{
			name:         "last seen in middle returns newer",
			executions:   execs,
			lastSeenUUID: "bbb",
			wantUUIDs:    []string{"aaa"},
		},
		{
			name:         "last seen is oldest returns all but last",
			executions:   execs,
			lastSeenUUID: "ccc",
			wantUUIDs:    []string{"aaa", "bbb"},
		},
		{
			name:         "last seen not in page returns all",
			executions:   execs,
			lastSeenUUID: "zzz",
			wantUUIDs:    []string{"aaa", "bbb", "ccc"},
		},
		{
			name:         "empty executions",
			executions:   nil,
			lastSeenUUID: "aaa",
			wantUUIDs:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := filterNew(tt.executions, tt.lastSeenUUID)
			var gotUUIDs []string
			for _, e := range result {
				gotUUIDs = append(gotUUIDs, e.UUID)
			}
			assert.Equal(t, tt.wantUUIDs, gotUUIDs)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"zero", 0, "0ms"},
		{"sub-second", 500 * time.Millisecond, "500ms"},
		{"one second", time.Second, "1.0s"},
		{"fractional seconds", 2300 * time.Millisecond, "2.3s"},
		{"large", 90 * time.Second, "90.0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, formatDuration(tt.duration))
		})
	}
}

func TestShortUUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		uuid string
		want string
	}{
		{"full uuid", "67e9ddbb-6531-4621-b990-3fdb7b518846", "67e9ddbb"},
		{"exactly 8", "12345678", "12345678"},
		{"short", "abc", "abc"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, shortUUID(tt.uuid))
		})
	}
}

// TestExecute tests are not parallel because they capture os.Stdout.
func TestExecute(t *testing.T) {
	t.Run("shows executions with error details", func(t *testing.T) {
		ts := newMockGraphQL(t, mockConfig{
			workflows: []map[string]any{
				{"uuid": "wf-1", "name": "test-workflow", "status": "ACTIVE"},
			},
			executions: []map[string]any{
				{
					"uuid":       "exec-2",
					"status":     "FAILURE",
					"startedAt":  "2026-02-12T16:01:00Z",
					"finishedAt": "2026-02-12T16:01:01Z",
				},
				{
					"uuid":       "exec-1",
					"status":     "SUCCESS",
					"startedAt":  "2026-02-12T16:00:00Z",
					"finishedAt": "2026-02-12T16:00:02Z",
				},
			},
			events: []map[string]any{
				{
					"capabilityID": "confidential-http@1.0.0",
					"status":       "failure",
					"errors":       []map[string]any{{"error": "connection refused", "count": 7}},
				},
			},
		})
		defer ts.Close()

		output := captureStdout(t, func() {
			h := newTestHandler(ts.URL, "test-workflow", false, 10)
			err := h.Execute(context.Background())
			require.NoError(t, err)
		})

		assert.Contains(t, output, "test-workflow")
		assert.Contains(t, output, "TIMESTAMP")
		assert.Contains(t, output, "success")
		assert.Contains(t, output, "failure")
		assert.Contains(t, output, "connection refused")
		assert.Contains(t, output, "confidential-http@1.0.0")

		// Verify chronological order (oldest first)
		successIdx := strings.Index(output, "success")
		failureIdx := strings.Index(output, "failure")
		assert.Greater(t, failureIdx, successIdx, "oldest execution should appear first")
	})

	t.Run("long error is truncated keeping tail", func(t *testing.T) {
		longErr := "failed to execute enclave request. enclave ID: abc123, error: attestation validation failed for ExecuteBatch: expected PCR0 deadbeef, got cafebabe"
		ts := newMockGraphQL(t, mockConfig{
			workflows: []map[string]any{
				{"uuid": "wf-1", "name": "test-workflow", "status": "ACTIVE"},
			},
			executions: []map[string]any{
				{
					"uuid":       "exec-1",
					"status":     "FAILURE",
					"startedAt":  "2026-02-12T16:00:00Z",
					"finishedAt": "2026-02-12T16:00:01Z",
				},
			},
			events: []map[string]any{
				{
					"capabilityID": "confidential-http@1.0.0",
					"status":       "failure",
					"errors":       []map[string]any{{"error": longErr, "count": 1}},
				},
			},
		})
		defer ts.Close()

		output := captureStdout(t, func() {
			h := newTestHandler(ts.URL, "test-workflow", false, 10)
			err := h.Execute(context.Background())
			require.NoError(t, err)
		})

		// Head (beginning) should be present
		assert.Contains(t, output, "failed to execute enclave")
		// Tail (last 40%) should survive truncation
		assert.Contains(t, output, "expected PCR0 deadbeef, got cafebabe")
		// Middle should be elided
		assert.Contains(t, output, "...")
	})

	t.Run("workflow not found", func(t *testing.T) {
		ts := newMockGraphQL(t, mockConfig{})
		defer ts.Close()

		h := newTestHandler(ts.URL, "nonexistent", false, 10)
		err := h.Execute(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no workflow found")
	})

	t.Run("partial name match shows suggestions", func(t *testing.T) {
		ts := newMockGraphQL(t, mockConfig{
			workflows: []map[string]any{
				{"uuid": "wf-1", "name": "my-workflow-staging", "status": "ACTIVE"},
				{"uuid": "wf-2", "name": "my-workflow-prod", "status": "ACTIVE"},
			},
		})
		defer ts.Close()

		h := newTestHandler(ts.URL, "my-workflow", false, 10)
		err := h.Execute(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no exact match")
		assert.Contains(t, err.Error(), "my-workflow-staging")
		assert.Contains(t, err.Error(), "my-workflow-prod")
	})

	t.Run("no executions", func(t *testing.T) {
		ts := newMockGraphQL(t, mockConfig{
			workflows: []map[string]any{
				{"uuid": "wf-1", "name": "test-workflow", "status": "ACTIVE"},
			},
		})
		defer ts.Close()

		output := captureStdout(t, func() {
			h := newTestHandler(ts.URL, "test-workflow", false, 10)
			err := h.Execute(context.Background())
			require.NoError(t, err)
		})

		assert.Contains(t, output, "No executions found")
	})

	t.Run("graphql error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]any{{"message": "unauthorized"}},
			})
		}))
		defer ts.Close()

		h := newTestHandler(ts.URL, "test-workflow", false, 10)
		err := h.Execute(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to search for workflow")
	})
}

// Test helpers

type mockConfig struct {
	workflows  []map[string]any
	executions []map[string]any
	events     []map[string]any
}

func newMockGraphQL(t *testing.T, cfg mockConfig) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(bodyStr, "FindWorkflow"):
			wfs := cfg.workflows
			if wfs == nil {
				wfs = []map[string]any{}
			}
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"workflows": map[string]any{
						"data":  wfs,
						"count": len(wfs),
					},
				},
			})

		case strings.Contains(bodyStr, "GetExecutions"):
			execs := cfg.executions
			if execs == nil {
				execs = []map[string]any{}
			}
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"workflowExecutions": map[string]any{
						"data":  execs,
						"count": len(execs),
					},
				},
			})

		case strings.Contains(bodyStr, "GetEvents"):
			evts := cfg.events
			if evts == nil {
				evts = []map[string]any{}
			}
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"workflowExecutionEvents": map[string]any{
						"data": evts,
					},
				},
			})

		default:
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
}

func newTestHandler(serverURL, workflowName string, follow bool, limit int) *handler {
	logger := zerolog.New(io.Discard)
	return &handler{
		log:         &logger,
		credentials: &credentials.Credentials{},
		environmentSet: &environments.EnvironmentSet{
			GraphQLURL: serverURL,
		},
		workflowName: workflowName,
		follow:       follow,
		limit:        limit,
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w
	t.Cleanup(func() { os.Stdout = oldStdout })

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	io.Copy(&buf, r)
	return buf.String()
}

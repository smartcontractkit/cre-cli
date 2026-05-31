package execution_test

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

	"github.com/smartcontractkit/cre-cli/cmd/workflow/execution"
	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

// ---- helpers ----

func nopLogger() *zerolog.Logger { l := zerolog.Nop(); return &l }

func credsAndEnv(serverURL string) (*credentials.Credentials, *environments.EnvironmentSet) {
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"}
	env := &environments.EnvironmentSet{GraphQLURL: serverURL}
	return creds, env
}

func wdcFor(t *testing.T, serverURL string) *workflowdataclient.Client {
	t.Helper()
	creds, env := credsAndEnv(serverURL)
	gql := graphqlclient.New(creds, env, nopLogger())
	return workflowdataclient.New(gql, nopLogger())
}

func rtCtxFor(t *testing.T, serverURL string) *runtime.Context {
	t.Helper()
	creds, env := credsAndEnv(serverURL)
	return &runtime.Context{
		Logger:         nopLogger(),
		Credentials:    creds,
		EnvironmentSet: env,
	}
}

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

// gqlRespond writes a standard GraphQL data envelope.
func gqlRespond(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": payload})
}

// ---- newList tests ----

func TestList_MissingCredentials(t *testing.T) {
	t.Parallel()
	ctx := &runtime.Context{Logger: nopLogger()}
	h := execution.NewListHandlerWithClient(ctx, wdcFor(t, "http://unused"))
	err := h.Execute(context.Background(), execution.ListInputs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials not available")
}

func TestList_InvalidStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflows": map[string]any{"data": []any{}, "count": 0},
		})
	}))
	t.Cleanup(srv.Close)

	cmd := execution.New(rtCtxFor(t, srv.URL))
	cmd.SetArgs([]string{"list", "--status", "RUNNING"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RUNNING")
	assert.Contains(t, err.Error(), "not valid")
}

func TestList_ByUUID_JSON(t *testing.T) {

	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)
	finished := time.Date(2026, 5, 29, 14, 0, 17, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflowExecutions": map[string]any{
				"count": 1,
				"data": []any{
					map[string]any{
						"uuid":         "exec-uuid-1",
						"workflowUUID": "wf-uuid-1",
						"workflowName": "my-workflow",
						"status":       "FAILURE",
						"startedAt":    started.Format(time.RFC3339),
						"finishedAt":   finished.Format(time.RFC3339),
						"creditUsed":   "0.05",
						"errors":       []any{},
					},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	rtCtx := rtCtxFor(t, srv.URL)
	h := execution.NewListHandlerWithClient(rtCtx, wdcFor(t, srv.URL))

	wfUUID := "wf-uuid-1"
	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execution.ListInputs{
			WorkflowUUID: &wfUUID,
			OutputFormat: "json",
		})
		require.NoError(t, err)
	})

	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result, 1)
	assert.Equal(t, "exec-uuid-1", result[0]["uuid"])
	assert.Equal(t, "FAILURE", result[0]["status"])
	assert.Equal(t, "0.05", result[0]["creditUsed"])
}

func TestList_ByName_ResolvesActiveWorkflow(t *testing.T) {

	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query, _ := body["query"].(string)

		if strings.Contains(query, "ListWorkflows") {
			gqlRespond(w, map[string]any{
				"workflows": map[string]any{
					"count": 1,
					"data": []any{
						map[string]any{
							"name":           "my-workflow",
							"workflowId":     "wf-uuid-active",
							"ownerAddress":   "0xowner",
							"status":         "ACTIVE",
							"workflowSource": "private",
						},
					},
				},
			})
			return
		}

		gqlRespond(w, map[string]any{
			"workflowExecutions": map[string]any{
				"count": 1,
				"data": []any{
					map[string]any{
						"uuid":         "exec-uuid-1",
						"workflowUUID": "wf-uuid-active",
						"workflowName": "my-workflow",
						"status":       "SUCCESS",
						"startedAt":    started.Format(time.RFC3339),
						"errors":       []any{},
					},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	rtCtx := rtCtxFor(t, srv.URL)
	h := execution.NewListHandlerWithClient(rtCtx, wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.ExecuteWithArg(context.Background(), "my-workflow", execution.ListInputs{OutputFormat: "json"})
		require.NoError(t, err)
	})

	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result, 1)
	assert.Equal(t, "wf-uuid-active", result[0]["workflowUUID"])
}

func TestList_NoArg_ListsAll(t *testing.T) {

	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflowExecutions": map[string]any{
				"count": 2,
				"data": []any{
					map[string]any{
						"uuid": "exec-1", "workflowUUID": "wf-1", "workflowName": "alpha",
						"status": "SUCCESS", "startedAt": started.Format(time.RFC3339), "errors": []any{},
					},
					map[string]any{
						"uuid": "exec-2", "workflowUUID": "wf-2", "workflowName": "beta",
						"status": "FAILURE", "startedAt": started.Format(time.RFC3339), "errors": []any{},
					},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	rtCtx := rtCtxFor(t, srv.URL)
	h := execution.NewListHandlerWithClient(rtCtx, wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execution.ListInputs{OutputFormat: "json"})
		require.NoError(t, err)
	})

	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Len(t, result, 2)
}

// ---- newStatus tests ----

func TestStatus_MissingCredentials(t *testing.T) {
	t.Parallel()
	ctx := &runtime.Context{Logger: nopLogger()}
	h := execution.NewStatusHandlerWithClient(ctx, wdcFor(t, "http://unused"))
	err := h.Execute(context.Background(), execution.StatusInputs{ExecutionUUID: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials not available")
}

func TestStatus_NotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflowExecution": map[string]any{"data": nil},
		})
	}))
	t.Cleanup(srv.Close)

	rtCtx := rtCtxFor(t, srv.URL)
	h := execution.NewStatusHandlerWithClient(rtCtx, wdcFor(t, srv.URL))
	err := h.Execute(context.Background(), execution.StatusInputs{ExecutionUUID: "missing-uuid"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStatus_FailureShowsErrors(t *testing.T) {

	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)
	finished := time.Date(2026, 5, 29, 14, 0, 17, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflowExecution": map[string]any{
				"data": map[string]any{
					"uuid":         "exec-uuid-1",
					"workflowUUID": "wf-uuid-1",
					"workflowName": "Price-Feed",
					"status":       "FAILURE",
					"startedAt":    started.Format(time.RFC3339),
					"finishedAt":   finished.Format(time.RFC3339),
					"errors": []any{
						map[string]any{"error": "Invalid JSON: unexpected char", "count": 1},
					},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	rtCtx := rtCtxFor(t, srv.URL)
	h := execution.NewStatusHandlerWithClient(rtCtx, wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execution.StatusInputs{ExecutionUUID: "exec-uuid-1", OutputFormat: "json"})
		require.NoError(t, err)
	})

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "FAILURE", result["status"])
	errs, _ := result["errors"].([]any)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].(map[string]any)["error"], "Invalid JSON")
}

// ---- newEvents tests ----

func TestEvents_MissingCredentials(t *testing.T) {
	t.Parallel()
	ctx := &runtime.Context{Logger: nopLogger()}
	h := execution.NewEventsHandlerWithClient(ctx, wdcFor(t, "http://unused"))
	err := h.Execute(context.Background(), execution.EventsInputs{ExecutionUUID: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials not available")
}

func TestEvents_JSON(t *testing.T) {

	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)
	finished := time.Date(2026, 5, 29, 14, 0, 7, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflowExecutionEvents": map[string]any{
				"data": []any{
					map[string]any{
						"capabilityID": "FetchData",
						"status":       "SUCCESS",
						"method":       "GET",
						"startedAt":    started.Format(time.RFC3339),
						"finishedAt":   finished.Format(time.RFC3339),
						"errors":       []any{},
					},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	rtCtx := rtCtxFor(t, srv.URL)
	h := execution.NewEventsHandlerWithClient(rtCtx, wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execution.EventsInputs{ExecutionUUID: "exec-1", OutputFormat: "json"})
		require.NoError(t, err)
	})

	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result, 1)
	assert.Equal(t, "FetchData", result[0]["capabilityID"])
	assert.Equal(t, "GET", result[0]["method"])
	assert.Equal(t, "2s", result[0]["duration"])
}

// ---- newLogs tests ----

func TestLogs_MissingCredentials(t *testing.T) {
	t.Parallel()
	ctx := &runtime.Context{Logger: nopLogger()}
	h := execution.NewLogsHandlerWithClient(ctx, wdcFor(t, "http://unused"))
	err := h.Execute(context.Background(), execution.LogsInputs{ExecutionUUID: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials not available")
}

func TestLogs_NodeFilter_ClientSide(t *testing.T) {

	ts := time.Date(2026, 5, 29, 14, 0, 8, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflowExecutionLogs": map[string]any{
				"data": []any{
					map[string]any{"nodeID": "ProcessData", "message": "Starting transformation", "timestamp": ts.Format(time.RFC3339)},
					map[string]any{"nodeID": "FetchData", "message": "HTTP GET called", "timestamp": ts.Format(time.RFC3339)},
					map[string]any{"nodeID": "ProcessData", "message": "Failed to parse", "timestamp": ts.Format(time.RFC3339)},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	rtCtx := rtCtxFor(t, srv.URL)
	h := execution.NewLogsHandlerWithClient(rtCtx, wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execution.LogsInputs{
			ExecutionUUID: "exec-1",
			NodeFilter:    "ProcessData",
			OutputFormat:  "json",
		})
		require.NoError(t, err)
	})

	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	// Only ProcessData lines should appear
	require.Len(t, result, 2)
	for _, row := range result {
		assert.Equal(t, "ProcessData", row["nodeID"])
	}
}

func TestLogs_NoFilter_ReturnsAll(t *testing.T) {

	ts := time.Date(2026, 5, 29, 14, 0, 8, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflowExecutionLogs": map[string]any{
				"data": []any{
					map[string]any{"nodeID": "A", "message": "msg1", "timestamp": ts.Format(time.RFC3339)},
					map[string]any{"nodeID": "B", "message": "msg2", "timestamp": ts.Format(time.RFC3339)},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	rtCtx := rtCtxFor(t, srv.URL)
	h := execution.NewLogsHandlerWithClient(rtCtx, wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execution.LogsInputs{ExecutionUUID: "exec-1", OutputFormat: "json"})
		require.NoError(t, err)
	})

	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Len(t, result, 2)
}

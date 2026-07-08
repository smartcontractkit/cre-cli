package wfstatus_test

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

	"github.com/smartcontractkit/cre-cli/cmd/workflow/wfstatus"
	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func nopLogger() *zerolog.Logger { l := zerolog.Nop(); return &l }

func testSettings(workflowName, owner string) *settings.Settings {
	s := &settings.Settings{
		User: settings.UserSettings{TargetName: "staging"},
	}
	s.Workflow.UserWorkflowSettings.WorkflowName = workflowName
	s.Workflow.UserWorkflowSettings.WorkflowOwnerAddress = owner
	return s
}

func wdcFor(t *testing.T, serverURL string) *workflowdataclient.Client {
	t.Helper()
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"}
	env := &environments.EnvironmentSet{GraphQLURL: serverURL}
	gql := graphqlclient.New(creds, env, nopLogger())
	return workflowdataclient.New(gql, nopLogger())
}

func rtCtxFor(t *testing.T, serverURL string, s *settings.Settings) *runtime.Context {
	t.Helper()
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"}
	env := &environments.EnvironmentSet{GraphQLURL: serverURL}
	return &runtime.Context{Logger: nopLogger(), Credentials: creds, EnvironmentSet: env, Settings: s}
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

func gqlRespond(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": payload})
}

func gqlError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]any{{"message": msg}},
	})
}

func TestStatus_MissingCredentials(t *testing.T) {
	t.Parallel()
	ctx := &runtime.Context{Logger: nopLogger(), Settings: testSettings("my-workflow", "0xowner")}
	h := wfstatus.NewHandlerWithClient(ctx, wdcFor(t, "http://unused"))
	err := h.Execute(context.Background(), wfstatus.Inputs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials not available")
}

func TestStatus_MissingSettings(t *testing.T) {
	t.Parallel()
	ctx := &runtime.Context{
		Logger:      nopLogger(),
		Credentials: &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"},
	}
	h := wfstatus.NewHandlerWithClient(ctx, wdcFor(t, "http://unused"))
	err := h.Execute(context.Background(), wfstatus.Inputs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workflow settings not loaded")
}

func TestStatus_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflows": map[string]any{"data": []any{}, "count": 0},
		})
	}))
	t.Cleanup(srv.Close)

	s := testSettings("missing-workflow", "0xowner")
	h := wfstatus.NewHandlerWithClient(rtCtxFor(t, srv.URL, s), wdcFor(t, srv.URL))
	err := h.Execute(context.Background(), wfstatus.Inputs{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no workflow found with name "missing-workflow"`)
}

func TestStatus_ResolvesByTargetWorkflowNameAndOwner(t *testing.T) {
	t.Parallel()
	const owner = "c96ca1860ed10e4a484a3f1b39b86769ae7e9772"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query, _ := body["query"].(string)

		switch {
		case strings.Contains(query, "ListWorkflows"):
			vars, _ := body["variables"].(map[string]any)
			input, _ := vars["input"].(map[string]any)
			owners, _ := input["workflowOwnerAddress"].([]any)
			require.Len(t, owners, 1)
			assert.Equal(t, owner, owners[0])
			assert.Equal(t, "private-launch-test-production", input["search"])

			gqlRespond(w, map[string]any{
				"workflows": map[string]any{
					"count": 1,
					"data": []any{
						map[string]any{
							"uuid":           "wf-uuid-1",
							"name":           "private-launch-test-production",
							"workflowId":     "abc123onchain",
							"ownerAddress":   owner,
							"status":         "ACTIVE",
							"workflowSource": "private",
						},
					},
				},
			})
		case strings.Contains(query, "GetWorkflow"):
			gqlRespond(w, map[string]any{
				"workflow": map[string]any{
					"data": map[string]any{
						"uuid":           "wf-uuid-1",
						"name":           "private-launch-test-production",
						"workflowId":     "abc123onchain",
						"ownerAddress":   owner,
						"status":         "ACTIVE",
						"workflowSource": "private",
						"executionCount": 0,
						"executionCountByStatus": map[string]any{
							"success": 0,
							"failure": 0,
						},
					},
				},
			})
		case strings.Contains(query, "GetLatestDeployment"):
			gqlRespond(w, map[string]any{"workflowDeployments": map[string]any{"data": []any{}}})
		case strings.Contains(query, "ListExecutions"):
			gqlRespond(w, map[string]any{"workflowExecutions": map[string]any{"count": 0, "data": []any{}}})
		default:
			gqlRespond(w, map[string]any{})
		}
	}))
	t.Cleanup(srv.Close)

	s := testSettings("private-launch-test-production", owner)
	h := wfstatus.NewHandlerWithClient(rtCtxFor(t, srv.URL, s), wdcFor(t, srv.URL))
	err := h.Execute(context.Background(), wfstatus.Inputs{OutputFormat: "json"})
	require.NoError(t, err)
}

func TestStatus_JSONOutput(t *testing.T) {
	registered := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	executed := time.Date(2026, 1, 15, 8, 30, 0, 0, time.UTC)
	deployed := time.Date(2026, 1, 10, 11, 55, 0, 0, time.UTC)
	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)
	finished := time.Date(2026, 5, 29, 14, 0, 17, 0, time.UTC)
	txHash := "0xabc123"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query, _ := body["query"].(string)

		switch {
		case strings.Contains(query, "ListWorkflows"):
			gqlRespond(w, map[string]any{
				"workflows": map[string]any{
					"count": 1,
					"data": []any{
						map[string]any{
							"uuid":           "wf-uuid-1",
							"name":           "my-workflow",
							"workflowId":     "abc123onchain",
							"ownerAddress":   "0xowner",
							"status":         "ACTIVE",
							"workflowSource": "private",
						},
					},
				},
			})
		case strings.Contains(query, "GetWorkflow"):
			gqlRespond(w, map[string]any{
				"workflow": map[string]any{
					"data": map[string]any{
						"uuid":           "wf-uuid-1",
						"name":           "my-workflow",
						"workflowId":     "abc123onchain",
						"ownerAddress":   "0xowner",
						"status":         "ACTIVE",
						"workflowSource": "private",
						"registeredAt":   registered.Format(time.RFC3339),
						"executedAt":     executed.Format(time.RFC3339),
						"executionCount": 42,
						"executionCountByStatus": map[string]any{
							"success": 40,
							"failure": 2,
						},
					},
				},
			})
		case strings.Contains(query, "GetLatestDeployment"):
			gqlRespond(w, map[string]any{
				"workflowDeployments": map[string]any{
					"data": []any{
						map[string]any{
							"uuid":       "dep-uuid-1",
							"workflowID": "abc123onchain",
							"status":     "SUCCESS",
							"deployedAt": deployed.Format(time.RFC3339),
							"txHash":     txHash,
						},
					},
				},
			})
		case strings.Contains(query, "ListExecutions"):
			gqlRespond(w, map[string]any{
				"workflowExecutions": map[string]any{
					"count": 1,
					"data": []any{
						map[string]any{
							"uuid":         "exec-uuid-1",
							"workflowUUID": "wf-uuid-1",
							"workflowName": "my-workflow",
							"status":       "SUCCESS",
							"startedAt":    started.Format(time.RFC3339),
							"finishedAt":   finished.Format(time.RFC3339),
							"errors":       []any{},
						},
					},
				},
			})
		default:
			t.Errorf("unexpected query: %s", query)
			gqlRespond(w, map[string]any{})
		}
	}))
	t.Cleanup(srv.Close)

	s := testSettings("my-workflow", "0xowner")
	h := wfstatus.NewHandlerWithClient(rtCtxFor(t, srv.URL, s), wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), wfstatus.Inputs{
			OutputFormat: "json",
		})
		require.NoError(t, err)
	})

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))

	workflow, ok := result["workflow"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "my-workflow", workflow["name"])
	assert.Equal(t, "ACTIVE", workflow["status"])
	assert.Equal(t, float64(42), workflow["executionCount"])
	assert.Equal(t, float64(40), workflow["successCount"])
	assert.Equal(t, float64(2), workflow["failureCount"])

	deployment, ok := result["deployment"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "SUCCESS", deployment["status"])
	assert.Equal(t, txHash, deployment["txHash"])

	lastExec, ok := result["lastExecution"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "exec-uuid-1", lastExec["uuid"])
	assert.Equal(t, "SUCCESS", lastExec["status"])
}

func TestStatus_ContinuesWhenDeploymentUnavailable(t *testing.T) {
	registered := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query, _ := body["query"].(string)

		switch {
		case strings.Contains(query, "ListWorkflows"):
			gqlRespond(w, map[string]any{
				"workflows": map[string]any{
					"count": 1,
					"data": []any{
						map[string]any{
							"uuid":           "wf-uuid-1",
							"name":           "my-workflow",
							"workflowId":     "abc123onchain",
							"ownerAddress":   "0xowner",
							"status":         "ACTIVE",
							"workflowSource": "private",
						},
					},
				},
			})
		case strings.Contains(query, "GetWorkflow"):
			gqlRespond(w, map[string]any{
				"workflow": map[string]any{
					"data": map[string]any{
						"uuid":           "wf-uuid-1",
						"name":           "my-workflow",
						"workflowId":     "abc123onchain",
						"ownerAddress":   "0xowner",
						"status":         "ACTIVE",
						"workflowSource": "private",
						"registeredAt":   registered.Format(time.RFC3339),
						"executionCount": 0,
						"executionCountByStatus": map[string]any{
							"success": 0,
							"failure": 0,
						},
					},
				},
			})
		case strings.Contains(query, "GetLatestDeployment"):
			gqlError(w, "deployment service unavailable")
		case strings.Contains(query, "ListExecutions"):
			gqlRespond(w, map[string]any{
				"workflowExecutions": map[string]any{"count": 0, "data": []any{}},
			})
		default:
			gqlRespond(w, map[string]any{})
		}
	}))
	t.Cleanup(srv.Close)

	s := testSettings("my-workflow", "0xowner")
	h := wfstatus.NewHandlerWithClient(rtCtxFor(t, srv.URL, s), wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), wfstatus.Inputs{
			OutputFormat: "json",
		})
		require.NoError(t, err)
	})

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Contains(t, result, "workflow")
	assert.NotContains(t, result, "deployment")
}

func TestStatus_InvalidOutputFormat(t *testing.T) {
	t.Parallel()
	cmd := wfstatus.New(rtCtxFor(t, "http://unused", testSettings("my-workflow", "0xowner")))
	cmd.SetArgs([]string{"./my-workflow", "--output", "csv"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "csv")
}

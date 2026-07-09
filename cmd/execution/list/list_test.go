package list_test

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

	"github.com/smartcontractkit/cre-cli/cmd/execution"
	execList "github.com/smartcontractkit/cre-cli/cmd/execution/list"
	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

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

func gqlRespond(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": payload})
}

func TestList_MissingCredentials(t *testing.T) {
	t.Parallel()
	ctx := &runtime.Context{Logger: nopLogger()}
	h := execList.NewHandlerWithClient(ctx, wdcFor(t, "http://unused"))
	err := h.Execute(context.Background(), execList.Inputs{})
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

func TestList_JSONOutput(t *testing.T) {
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
	h := execList.NewHandlerWithClient(rtCtx, wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execList.Inputs{OutputFormat: "json"})
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
							"uuid":           "wf-uuid-active",
							"name":           "my-workflow",
							"workflowId":     "abc123onchain",
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
	h := execList.NewHandlerWithClient(rtCtx, wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execList.Inputs{
			WorkflowRef:  "my-workflow",
			OutputFormat: "json",
		})
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
	h := execList.NewHandlerWithClient(rtCtx, wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execList.Inputs{OutputFormat: "json"})
		require.NoError(t, err)
	})

	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Len(t, result, 2)
}

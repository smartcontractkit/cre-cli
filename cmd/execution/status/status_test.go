package status_test

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

	execStatus "github.com/smartcontractkit/cre-cli/cmd/execution/status"
	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func nopLogger() *zerolog.Logger { l := zerolog.Nop(); return &l }

func wdcFor(t *testing.T, serverURL string) *workflowdataclient.Client {
	t.Helper()
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"}
	env := &environments.EnvironmentSet{GraphQLURL: serverURL}
	gql := graphqlclient.New(creds, env, nopLogger())
	return workflowdataclient.New(gql, nopLogger())
}

func rtCtxFor(t *testing.T, serverURL string) *runtime.Context {
	t.Helper()
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"}
	env := &environments.EnvironmentSet{GraphQLURL: serverURL}
	return &runtime.Context{Logger: nopLogger(), Credentials: creds, EnvironmentSet: env}
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

func TestStatus_MissingCredentials(t *testing.T) {
	t.Parallel()
	ctx := &runtime.Context{Logger: nopLogger()}
	h := execStatus.NewHandlerWithClient(ctx, wdcFor(t, "http://unused"))
	err := h.Execute(context.Background(), execStatus.Inputs{ExecutionRef: "x"})
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

	h := execStatus.NewHandlerWithClient(rtCtxFor(t, srv.URL), wdcFor(t, srv.URL))
	err := h.Execute(context.Background(), execStatus.Inputs{ExecutionRef: "00000000-0000-0000-0000-000000000001"})
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

	h := execStatus.NewHandlerWithClient(rtCtxFor(t, srv.URL), wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execStatus.Inputs{
			ExecutionRef: "05ace5cf-85ae-448b-9f42-270d42974d35",
			OutputFormat: "json",
		})
		require.NoError(t, err)
	})

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Equal(t, "FAILURE", result["status"])
	errs, _ := result["errors"].([]any)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].(map[string]any)["error"], "Invalid JSON")
	assert.NotContains(t, out, "Debug further:")
	assert.NotContains(t, out, "cre execution")
}

func TestStatus_TableShowsDebugHints(t *testing.T) {
	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)
	finished := time.Date(2026, 5, 29, 14, 0, 17, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlRespond(w, map[string]any{
			"workflowExecution": map[string]any{
				"data": map[string]any{
					"uuid":         "exec-uuid-1",
					"workflowUUID": "wf-uuid-1",
					"workflowName": "Price-Feed",
					"workflowId":   "abc123onchain",
					"status":       "SUCCESS",
					"startedAt":    started.Format(time.RFC3339),
					"finishedAt":   finished.Format(time.RFC3339),
					"errors":       []any{},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	h := execStatus.NewHandlerWithClient(rtCtxFor(t, srv.URL), wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execStatus.Inputs{
			ExecutionRef: "05ace5cf-85ae-448b-9f42-270d42974d35",
		})
		require.NoError(t, err)
	})

	assert.Contains(t, out, "Debug further:")
	assert.Contains(t, out, "cre execution events")
	assert.Contains(t, out, "cre execution logs")
}

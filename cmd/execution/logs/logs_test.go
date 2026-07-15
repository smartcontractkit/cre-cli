package logs_test

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

	execLogs "github.com/smartcontractkit/cre-cli/cmd/execution/logs"
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

func TestLogs_MissingCredentials(t *testing.T) {
	t.Parallel()
	ctx := &runtime.Context{Logger: nopLogger()}
	h := execLogs.NewHandlerWithClient(ctx, wdcFor(t, "http://unused"))
	err := h.Execute(context.Background(), execLogs.Inputs{ExecutionRef: "x"})
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

	h := execLogs.NewHandlerWithClient(rtCtxFor(t, srv.URL), wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execLogs.Inputs{
			ExecutionRef: "05ace5cf-85ae-448b-9f42-270d42974d35",
			NodeFilter:   "ProcessData",
			OutputFormat: "json",
		})
		require.NoError(t, err)
	})

	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
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

	h := execLogs.NewHandlerWithClient(rtCtxFor(t, srv.URL), wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execLogs.Inputs{
			ExecutionRef: "05ace5cf-85ae-448b-9f42-270d42974d35",
			OutputFormat: "json",
		})
		require.NoError(t, err)
	})

	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	assert.Len(t, result, 2)
}

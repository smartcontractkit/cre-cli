package events_test

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

	execEvents "github.com/smartcontractkit/cre-cli/cmd/execution/events"
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

func TestEvents_MissingCredentials(t *testing.T) {
	t.Parallel()
	ctx := &runtime.Context{Logger: nopLogger()}
	h := execEvents.NewHandlerWithClient(ctx, wdcFor(t, "http://unused"))
	err := h.Execute(context.Background(), execEvents.Inputs{ExecutionRef: "x"})
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

	h := execEvents.NewHandlerWithClient(rtCtxFor(t, srv.URL), wdcFor(t, srv.URL))

	out := captureStdout(t, func() {
		err := h.Execute(context.Background(), execEvents.Inputs{
			ExecutionRef: "05ace5cf-85ae-448b-9f42-270d42974d35",
			OutputFormat: "json",
		})
		require.NoError(t, err)
	})

	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result, 1)
	assert.Equal(t, "FetchData", result[0]["capabilityID"])
	assert.Equal(t, "GET", result[0]["method"])
	assert.Equal(t, "2s", result[0]["duration"])
}

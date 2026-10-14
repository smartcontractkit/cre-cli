package whoami_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/whoami"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func TestHandlerExecute(t *testing.T) {
	tests := []struct {
		name           string
		graphqlHandler http.HandlerFunc
		wantErr        bool
		wantLogSnips   []string
	}{
		{
			name: "successful response",
			graphqlHandler: func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if strings.Contains(string(body), "getAccountDetails") && strings.Contains(string(body), "getOrganization") {
					resp := map[string]interface{}{
						"data": map[string]interface{}{
							"getAccountDetails": map[string]string{
								"username":     "alice",
								"emailAddress": "alice@example.com",
							},
							"getOrganization": map[string]string{
								"organizationID": "org-42",
								"displayName":    "Alice's Org",
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(resp); err != nil {
						t.Fatalf("failed to encode GraphQL response: %v", err)
					}
				} else {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
			},
			wantErr: false,
			wantLogSnips: []string{
				"Account Details",
				"Email:             alice@example.com",
				"Organization ID:   org-42",
				"Organization Name: Alice's Org",
			},
		},
		{
			name: "successful response - no account details (API key)",
			graphqlHandler: func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if strings.Contains(string(body), "getAccountDetails") && strings.Contains(string(body), "getOrganization") {
					resp := map[string]interface{}{
						"data": map[string]interface{}{
							"getOrganization": map[string]string{
								"organizationID": "org-42",
								"displayName":    "Alice's Org",
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(resp); err != nil {
						t.Fatalf("failed to encode GraphQL response: %v", err)
					}
				} else {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
			},
			wantErr: false,
			wantLogSnips: []string{
				"Account Details",
				"Organization ID:   org-42",
				"Organization Name: Alice's Org",
			},
		},
		{
			name: "graphql error",
			graphqlHandler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "boom", http.StatusInternalServerError)
			},
			wantErr:      true,
			wantLogSnips: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(tc.graphqlHandler)
			defer ts.Close()

			envSet := &environments.EnvironmentSet{
				GraphQLURL: ts.URL,
			}

			creds := &credentials.Credentials{}
			var buf bytes.Buffer
			logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

			rtCtx := &runtime.Context{
				Credentials:    creds,
				Logger:         &logger,
				EnvironmentSet: envSet,
			}

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			h := whoami.NewHandler(rtCtx)
			err := h.Execute(context.Background(), whoami.Inputs{})

			w.Close()
			os.Stdout = oldStdout
			var output strings.Builder
			_, _ = io.Copy(&output, r)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "graphql request failed") {
					t.Errorf("expected graphql-failure wrap, got %v", err)
				}
				if output.Len() > 0 {
					t.Errorf("did not expect logs on error, got %q", output.String())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				logs := output.String()
				for _, snippet := range tc.wantLogSnips {
					if !strings.Contains(logs, snippet) {
						t.Errorf("log output missing %q; full logs:\n%s", snippet, logs)
					}
				}
			}
		})
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

func runWhoami(t *testing.T, creds *credentials.Credentials, envName string, resp map[string]any, args ...string) (string, error) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": resp})
	}))
	t.Cleanup(ts.Close)

	logger := zerolog.Nop()
	cmd := whoami.New(&runtime.Context{
		Credentials:    creds,
		Logger:         &logger,
		EnvironmentSet: &environments.EnvironmentSet{GraphQLURL: ts.URL, EnvName: envName},
	})
	cmd.SetArgs(args)

	var err error
	out := captureStdout(t, func() { err = cmd.Execute() })
	return out, err
}

func TestWhoami_JSONOutput(t *testing.T) {
	out, err := runWhoami(t, &credentials.Credentials{}, "STAGING", map[string]any{
		"getAccountDetails": map[string]string{"emailAddress": "alice@example.com"},
		"getOrganization":   map[string]string{"organizationId": "org-42", "displayName": "Alice's Org"},
	}, "--json")
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got), "stdout must be only JSON, got %q", out)
	assert.Equal(t, map[string]any{
		"organizationId":   "org-42",
		"organizationName": "Alice's Org",
		"email":            "alice@example.com",
		"environment":      "STAGING",
	}, got)
}

func TestWhoami_JSONOutput_APIKeyOmitsEmail(t *testing.T) {
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"}
	out, err := runWhoami(t, creds, "", map[string]any{
		"getOrganization": map[string]string{"organizationId": "org-42", "displayName": "Alice's Org"},
	}, "--output", "json")
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got), "stdout must be only JSON, got %q", out)
	assert.NotContains(t, got, "email")
	assert.Equal(t, "org-42", got["organizationId"])
	assert.Equal(t, "Alice's Org", got["organizationName"])
	assert.Equal(t, "PRODUCTION", got["environment"])
}

func TestWhoami_InvalidOutputFormat(t *testing.T) {
	out, err := runWhoami(t, &credentials.Credentials{}, "", map[string]any{}, "--output", "csv")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "csv")
	assert.Empty(t, out)
}

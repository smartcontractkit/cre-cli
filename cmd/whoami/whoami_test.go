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
				"Account details retrieved:", "Email:             alice@example.com",
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
				"Account details retrieved:",
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
			err := h.Execute(context.Background())

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

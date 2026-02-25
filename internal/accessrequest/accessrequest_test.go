package accessrequest_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/accessrequest"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

func TestSubmitAccessRequest(t *testing.T) {
	tests := []struct {
		name           string
		useCase        string
		graphqlHandler http.HandlerFunc
		wantErr        bool
		wantErrMsg     string
	}{
		{
			name:    "successful request",
			useCase: "Building a cross-chain DeFi protocol",
			graphqlHandler: func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				bodyStr := string(body)

				if !strings.Contains(bodyStr, "requestDeploymentAccess") {
					t.Errorf("expected mutation requestDeploymentAccess in body, got: %s", bodyStr)
				}
				if !strings.Contains(bodyStr, "Building a cross-chain DeFi protocol") {
					t.Errorf("expected use case description in body, got: %s", bodyStr)
				}

				resp := map[string]interface{}{
					"data": map[string]interface{}{
						"requestDeploymentAccess": map[string]interface{}{
							"success": true,
							"message": nil,
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr: false,
		},
		{
			name:    "request denied with message",
			useCase: "some use case",
			graphqlHandler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]interface{}{
					"data": map[string]interface{}{
						"requestDeploymentAccess": map[string]interface{}{
							"success": false,
							"message": "organization is not eligible",
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			wantErrMsg: "organization is not eligible",
		},
		{
			name:    "request denied without message",
			useCase: "some use case",
			graphqlHandler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]interface{}{
					"data": map[string]interface{}{
						"requestDeploymentAccess": map[string]interface{}{
							"success": false,
							"message": nil,
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			wantErrMsg: "access request was not successful",
		},
		{
			name:    "graphql server error",
			useCase: "some use case",
			graphqlHandler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "internal server error", http.StatusInternalServerError)
			},
			wantErr:    true,
			wantErrMsg: "graphql request failed",
		},
		{
			name:    "graphql returns errors",
			useCase: "some use case",
			graphqlHandler: func(w http.ResponseWriter, r *http.Request) {
				resp := map[string]interface{}{
					"errors": []map[string]interface{}{
						{"message": "not authenticated"},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			},
			wantErr:    true,
			wantErrMsg: "graphql request failed",
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
			logger := zerolog.New(io.Discard)

			requester := accessrequest.NewRequester(creds, envSet, &logger)
			err := requester.SubmitAccessRequest(context.Background(), tc.useCase)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("expected error containing %q, got: %v", tc.wantErrMsg, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

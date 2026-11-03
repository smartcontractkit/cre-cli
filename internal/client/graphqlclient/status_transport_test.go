package graphqlclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

func runAgainst(t *testing.T, status int, body string) (map[string]any, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	logger := zerolog.Nop()
	client := New(
		&credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"},
		&environments.EnvironmentSet{GraphQLURL: srv.URL},
		&logger,
	)
	var resp map[string]any
	err := client.Execute(context.Background(), graphql.NewRequest("query { x }"), &resp)
	return resp, err
}

func TestExecute_NonOKWithoutGraphQLErrorsReturnsHTTPStatusError(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		retryable bool
	}{
		{"gateway 503 json message", http.StatusServiceUnavailable, `{"message":"Service Unavailable"}`, true},
		{"rate limited empty object", http.StatusTooManyRequests, `{}`, true},
		{"bad gateway null data", http.StatusBadGateway, `{"data":null}`, true},
		{"forbidden json message", http.StatusForbidden, `{"message":"Forbidden"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runAgainst(t, tt.status, tt.body)
			var statusErr *HTTPStatusError
			if !errors.As(err, &statusErr) {
				t.Fatalf("expected HTTPStatusError, got %v", err)
			}
			if statusErr.StatusCode != tt.status {
				t.Errorf("status = %d, want %d", statusErr.StatusCode, tt.status)
			}
			if statusErr.Retryable() != tt.retryable {
				t.Errorf("Retryable() = %v, want %v", statusErr.Retryable(), tt.retryable)
			}
		})
	}
}

func TestExecute_NonOKWithGraphQLErrorsPreservesMessage(t *testing.T) {
	_, err := runAgainst(t, http.StatusUnauthorized, `{"errors":[{"message":"invalid api key"}]}`)
	if err == nil || err.Error() != "cre api error: invalid api key" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecute_OKResponseDecodes(t *testing.T) {
	resp, err := runAgainst(t, http.StatusOK, `{"data":{"x":"y"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp["x"] != "y" {
		t.Errorf("resp = %v", resp)
	}
}

func TestHTTPStatusError_TruncatesBody(t *testing.T) {
	_, err := runAgainst(t, http.StatusBadGateway, strings.Repeat("a", 2*maxErrorBodyBytes))
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected HTTPStatusError, got %v", err)
	}
	if len(statusErr.Body) != maxErrorBodyBytes+len("...") {
		t.Errorf("body length = %d", len(statusErr.Body))
	}
}

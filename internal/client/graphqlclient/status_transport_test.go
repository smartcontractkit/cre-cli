package graphqlclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestExecute_TransientStatusWithGraphQLErrorsIsRetryable(t *testing.T) {
	_, err := runAgainst(t, http.StatusServiceUnavailable, `{"errors":[{"message":"temporarily unavailable"}]}`)
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected HTTPStatusError, got %v", err)
	}
	if !statusErr.Retryable() || statusErr.Message != "temporarily unavailable" {
		t.Errorf("unexpected error: %+v", statusErr)
	}
}

func TestExecute_RawBodyExcludedFromError(t *testing.T) {
	_, err := runAgainst(t, http.StatusBadGateway, `{"password":"secret"}`)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("raw body leaked into error: %v", err)
	}
}

func TestExecute_OversizedErrorBodyIsBounded(t *testing.T) {
	_, err := runAgainst(t, http.StatusBadGateway, strings.Repeat("a", 4*maxErrorBodyBytes))
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected HTTPStatusError 502, got %v", err)
	}
}

func TestExecute_FollowsRedirects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new", http.StatusPermanentRedirect)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"x":"y"}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	logger := zerolog.Nop()
	client := New(
		&credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"},
		&environments.EnvironmentSet{GraphQLURL: srv.URL + "/old"},
		&logger,
	)
	var resp map[string]any
	if err := client.Execute(context.Background(), graphql.NewRequest("query { x }"), &resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp["x"] != "y" {
		t.Errorf("resp = %v", resp)
	}
}

func TestExecute_CancelledWhileReadingErrorBodyReturnsContextError(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":`))
		w.(http.Flusher).Flush()
		<-release
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	logger := zerolog.Nop()
	client := New(
		&credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"},
		&environments.EnvironmentSet{GraphQLURL: srv.URL},
		&logger,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var resp map[string]any
	err := client.Execute(ctx, graphql.NewRequest("query { x }"), &resp)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		t.Errorf("cancellation misreported as HTTP status error: %v", err)
	}
}

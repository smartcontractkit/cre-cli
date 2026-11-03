package authvalidation

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

const okBody = `{"data":{"getCreOrganizationInfo":{"orgId":"org-1","derivedWorkflowOwners":["0xabc"]}}}`

type reply struct {
	status int
	body   string
}

// newTestValidator serves replies in order, repeating the last one once exhausted.
func newTestValidator(t *testing.T, replies ...reply) (*Validator, *credentials.Credentials, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		i := int(calls.Add(1)) - 1
		if i >= len(replies) {
			i = len(replies) - 1
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(replies[i].status)
		_, _ = w.Write([]byte(replies[i].body))
	}))
	t.Cleanup(srv.Close)

	logger := zerolog.Nop()
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "k"}
	v := NewValidator(creds, &environments.EnvironmentSet{GraphQLURL: srv.URL}, &logger)
	v.retryBaseDelay = time.Millisecond
	return v, creds, &calls
}

func TestValidateCredentials_Success(t *testing.T) {
	v, creds, calls := newTestValidator(t, reply{http.StatusOK, okBody})
	result, err := v.ValidateCredentials(context.Background(), creds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OrgID != "org-1" || result.DerivedWorkflowOwner != "0xabc" || creds.OrgID != "org-1" {
		t.Errorf("unexpected result: %+v, creds.OrgID=%q", result, creds.OrgID)
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", calls.Load())
	}
}

func TestValidateCredentials_RecoversFromTransientFailures(t *testing.T) {
	tests := []struct {
		name  string
		first reply
	}{
		{"gateway 503", reply{http.StatusServiceUnavailable, `{"message":"Service Unavailable"}`}},
		{"rate limited", reply{http.StatusTooManyRequests, `{}`}},
		{"503 with graphql errors", reply{http.StatusServiceUnavailable, `{"errors":[{"message":"temporarily unavailable"}]}`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, creds, calls := newTestValidator(t, tt.first, reply{http.StatusOK, okBody})
			result, err := v.ValidateCredentials(context.Background(), creds)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.OrgID != "org-1" {
				t.Errorf("OrgID = %q", result.OrgID)
			}
			if calls.Load() != 2 {
				t.Errorf("calls = %d, want 2", calls.Load())
			}
		})
	}
}

func TestValidateCredentials_ReportsPersistentFailures(t *testing.T) {
	tests := []struct {
		name    string
		reply   reply
		calls   int32
		wantErr string
	}{
		{"null org info", reply{http.StatusOK, `{"data":{"getCreOrganizationInfo":null}}`}, 1, "returned no organization"},
		{"missing both fields", reply{http.StatusOK, `{"data":{"getCreOrganizationInfo":{"orgId":"","derivedWorkflowOwners":[]}}}`}, 1, "missing orgId, derivedWorkflowOwners"},
		{"missing owners only", reply{http.StatusOK, `{"data":{"getCreOrganizationInfo":{"orgId":"org-1","derivedWorkflowOwners":[]}}}`}, 1, "missing derivedWorkflowOwners"},
		{"persistent 502", reply{http.StatusBadGateway, `{"message":"Bad Gateway"}`}, defaultMaxAttempts, "HTTP 502"},
		{"graphql auth error not retried", reply{http.StatusUnauthorized, `{"errors":[{"message":"invalid api key"}]}`}, 1, "invalid api key"},
		{"403 not retried", reply{http.StatusForbidden, `{"message":"Forbidden"}`}, 1, "HTTP 403"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, creds, calls := newTestValidator(t, tt.reply)
			_, err := v.ValidateCredentials(context.Background(), creds)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
			if calls.Load() != tt.calls {
				t.Errorf("calls = %d, want %d", calls.Load(), tt.calls)
			}
		})
	}
}

func TestValidateCredentials_HTTPStatusErrorIsUnwrappable(t *testing.T) {
	v, creds, _ := newTestValidator(t, reply{http.StatusServiceUnavailable, `{}`})
	_, err := v.ValidateCredentials(context.Background(), creds)
	var statusErr *graphqlclient.HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected wrapped HTTPStatusError, got %v", err)
	}
}

func TestValidateCredentials_StopsRetryingWhenContextCancelled(t *testing.T) {
	v, creds, calls := newTestValidator(t, reply{http.StatusServiceUnavailable, `{}`})
	v.retryBaseDelay = time.Hour
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := v.ValidateCredentials(ctx, creds)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	var statusErr *graphqlclient.HTTPStatusError
	if !errors.As(err, &statusErr) {
		t.Errorf("expected last API error to be preserved, got %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", calls.Load())
	}
}

package graphqlclient

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/creconfig"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

func TestRedactSensitiveHeaders(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "redacts bearer token",
			input:    ">> headers: map[Authorization:[Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.longtoken.signature] Content-Type:[application/json]]",
			expected: ">> headers: map[Authorization:[[REDACTED]] Content-Type:[application/json]]",
		},
		{
			name:     "redacts api key",
			input:    ">> headers: map[Authorization:[Apikey sk_live_abc123xyz789] User-Agent:[cre-cli]]",
			expected: ">> headers: map[Authorization:[[REDACTED]] User-Agent:[cre-cli]]",
		},
		{
			name:     "no change for messages without authorization",
			input:    ">> query: mutation { createUser }",
			expected: ">> query: mutation { createUser }",
		},
		{
			name:     "no change for response messages",
			input:    "<< {\"data\":{\"user\":{\"id\":\"123\"}}}",
			expected: "<< {\"data\":{\"user\":{\"id\":\"123\"}}}",
		},
		{
			name:     "handles variables message",
			input:    ">> variables: map[email:test@example.com]",
			expected: ">> variables: map[email:test@example.com]",
		},
		{
			name:     "redacts short token",
			input:    ">> headers: map[Authorization:[Bearer abc]]",
			expected: ">> headers: map[Authorization:[[REDACTED]]]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSensitiveHeaders(tt.input)
			if result != tt.expected {
				t.Errorf("redactSensitiveHeaders(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExecute_ErrorPrefixReplacement(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// This will cause the machinebox/graphql client to return an error starting with "graphql: "
		_, _ = w.Write([]byte(`{"errors": [{"message": "DON family \"zone-a\" is not supported"}]}`))
	}))
	defer srv.Close()

	creds := &credentials.Credentials{
		AuthType: credentials.AuthTypeApiKey,
		APIKey:   "test-api-key",
	}
	envSet := &environments.EnvironmentSet{GraphQLURL: srv.URL}
	logger := zerolog.Nop()

	client := New(creds, envSet, &logger)

	req := graphql.NewRequest(`query { test }`)
	var resp interface{}

	err := client.Execute(context.Background(), req, &resp)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedErr := "cre api error: DON family \"zone-a\" is not supported"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestExecute_TokenRefreshClearsTenantsCache(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	contextPath := filepath.Join(tmpHome, creconfig.Dir, creconfig.ContextFile)
	if err := os.MkdirAll(filepath.Dir(contextPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contextPath, []byte("PRODUCTION:\n  tenant_id: \"1\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token": %q, "refresh_token": "new-refresh", "expires_in": 3600, "token_type": "Bearer"}`, fakeJWT(time.Now().Add(time.Hour)))
	}))
	defer authSrv.Close()

	gqlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"test": true}}`))
	}))
	defer gqlSrv.Close()

	creds := &credentials.Credentials{
		AuthType: credentials.AuthTypeBearer,
		Tokens: &credentials.CreLoginTokenSet{
			AccessToken:  fakeJWT(time.Now().Add(-time.Hour)), // expired, forces refresh
			RefreshToken: "old-refresh",
		},
	}
	envSet := &environments.EnvironmentSet{
		GraphQLURL: gqlSrv.URL,
		AuthBase:   authSrv.URL,
		ClientID:   "test-client",
	}
	logger := zerolog.Nop()

	client := New(creds, envSet, &logger)

	req := graphql.NewRequest(`query { test }`)
	var resp interface{}
	if err := client.Execute(context.Background(), req, &resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(contextPath); !os.IsNotExist(err) {
		t.Error("expected tenants cache to be cleared after token refresh")
	}
}

// fakeJWT builds an unsigned JWT-shaped token with the given expiration.
func fakeJWT(exp time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp.Unix())))
	return header + "." + payload + ".sig"
}

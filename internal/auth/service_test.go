package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/credentials"
	"github.com/smartcontractkit/dev-platform/internal/environments"
)

func testEnv(base string) *environments.EnvironmentSet {
	return &environments.EnvironmentSet{
		AuthBase: base,
		ClientID: "test-client",
		Audience: "",
	}
}

func TestRefreshToken_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != constants.AuthTokenPath {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"id_token":      "new-id",
			"refresh_token": "new-refresh",
			"expires_in":    1234,
			"token_type":    "Bearer",
		})
		if err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer ts.Close()

	svc := NewOAuthService(testEnv(ts.URL))
	old := &credentials.CreLoginTokenSet{RefreshToken: "old-refresh"}
	tok, err := svc.RefreshToken(context.Background(), old)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok.AccessToken != "new-access" || tok.RefreshToken != "new-refresh" {
		t.Errorf("unexpected token set: %+v", tok)
	}
}

func TestRefreshToken_NoRefreshToken(t *testing.T) {
	svc := NewOAuthService(testEnv("http://example.com"))
	_, err := svc.RefreshToken(context.Background(), &credentials.CreLoginTokenSet{})
	if err == nil || err.Error() != "no refresh token available" {
		t.Errorf("expected no refresh token error, got %v", err)
	}
}

func TestRefreshToken_HTTPError(t *testing.T) {
	svc := NewOAuthService(testEnv("http://localhost:0"))
	old := &credentials.CreLoginTokenSet{RefreshToken: "old-refresh"}
	_, err := svc.RefreshToken(context.Background(), old)
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestRevokeToken_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != constants.AuthRevokePath {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	svc := NewOAuthService(testEnv(ts.URL))
	err := svc.RevokeToken(context.Background(), "token-to-revoke")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRevokeToken_HTTPError(t *testing.T) {
	svc := NewOAuthService(testEnv("http://localhost:0"))
	err := svc.RevokeToken(context.Background(), "token")
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

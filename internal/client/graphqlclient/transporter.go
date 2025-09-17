package graphqlclient

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/smartcontractkit/cre-cli/internal/auth"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
)

type headerTransport struct {
	base         http.RoundTripper
	credentials  *credentials.Credentials
	authService  *auth.OAuthService
	extraHeaders http.Header
}

func newHeaderTransport(credentials *credentials.Credentials, authSvc *auth.OAuthService, base http.RoundTripper) *headerTransport {
	return &headerTransport{
		base:         base,
		credentials:  credentials,
		authService:  authSvc,
		extraHeaders: make(http.Header),
	}
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.credentials == nil {
		return nil, fmt.Errorf("credentials not provided")
	}

	clone := req.Clone(req.Context())
	t.injectHeaders(clone)

	resp, err := t.base.RoundTrip(clone)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode == http.StatusUnauthorized &&
		t.credentials.AuthType != credentials.AuthTypeApiKey &&
		t.credentials.Tokens != nil &&
		t.credentials.Tokens.RefreshToken != "" {

		newTokens, refreshErr := t.authService.RefreshToken(req.Context(), t.credentials.Tokens)
		if refreshErr != nil {
			return resp, fmt.Errorf("token refresh failed: %w", refreshErr)
		}
		t.credentials.Tokens = newTokens

		retry := req.Clone(req.Context())
		t.injectHeaders(retry)
		return t.base.RoundTrip(retry)
	}

	return resp, nil
}

func (t *headerTransport) injectHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Idempotency-Key", uuid.New().String())
	req.Header.Set("User-Agent", "cre-cli")

	switch t.credentials.AuthType {
	case credentials.AuthTypeApiKey:
		if t.credentials.APIKey != "" {
			req.Header.Set("Authorization", "Apikey "+t.credentials.APIKey)
		}
	default:
		if t.credentials.Tokens != nil && t.credentials.Tokens.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+t.credentials.Tokens.AccessToken)
		}
	}

	for k, vs := range t.extraHeaders {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}
}

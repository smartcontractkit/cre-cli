package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

// DefaultHTTPClient is used for token exchange when no client is supplied.
var DefaultHTTPClient = &http.Client{Timeout: 10 * time.Second}

// ExchangeAuthorizationCode exchanges an OAuth authorization code for tokens using
// environment credentials (AuthBase, ClientID) and PKCE code_verifier.
func ExchangeAuthorizationCode(ctx context.Context, httpClient *http.Client, env *environments.EnvironmentSet, code, codeVerifier string) (*credentials.CreLoginTokenSet, error) {
	if httpClient == nil {
		httpClient = DefaultHTTPClient
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", env.ClientID)
	form.Set("code", code)
	form.Set("redirect_uri", constants.AuthRedirectURI)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, env.AuthBase+constants.AuthTokenPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req) // #nosec G704 -- URL is from trusted environment config
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}

	var tokenSet credentials.CreLoginTokenSet
	if err := json.Unmarshal(body, &tokenSet); err != nil {
		return nil, fmt.Errorf("unmarshal token set: %w", err)
	}
	return &tokenSet, nil
}

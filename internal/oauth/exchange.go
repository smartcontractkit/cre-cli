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

// ExchangeAuthorizationCode exchanges an OAuth authorization code for tokens (PKCE).
// If oauthClientID is non-empty, it is used as client_id (must match the authorize URL).
// If oauthAuthServerBase is non-empty (scheme + host only), it is used as the token endpoint host;
// otherwise env.AuthBase is used (e.g. cre login builds the authorize URL from env).
func ExchangeAuthorizationCode(ctx context.Context, httpClient *http.Client, env *environments.EnvironmentSet, code, codeVerifier, oauthClientID, oauthAuthServerBase string) (*credentials.CreLoginTokenSet, error) {
	if httpClient == nil {
		httpClient = DefaultHTTPClient
	}
	clientID := env.ClientID
	if oauthClientID != "" {
		clientID = oauthClientID
	}
	authBase := env.AuthBase
	if oauthAuthServerBase != "" {
		authBase = oauthAuthServerBase
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("code", code)
	form.Set("redirect_uri", constants.AuthRedirectURI)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authBase+constants.AuthTokenPath, strings.NewReader(form.Encode()))
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

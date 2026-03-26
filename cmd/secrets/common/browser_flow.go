package common

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	rt "runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/machinebox/graphql"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/oauth"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const createVaultAuthURLMutation = `mutation CreateVaultAuthorizationUrl($request: VaultAuthorizationUrlRequest!) {
  createVaultAuthorizationUrl(request: $request) {
    url
  }
}`

// vaultPermissionForMethod returns the API permission name for the given vault operation.
func vaultPermissionForMethod(method string) (string, error) {
	switch method {
	case vaulttypes.MethodSecretsCreate:
		return "VAULT_PERMISSION_CREATE_SECRETS", nil
	case vaulttypes.MethodSecretsUpdate:
		return "VAULT_PERMISSION_UPDATE_SECRETS", nil
	default:
		return "", fmt.Errorf("unsupported method: %s", method)
	}
}

func digestHexString(digest [32]byte) string {
	return "0x" + hex.EncodeToString(digest[:])
}

// executeBrowserUpsert handles secrets create/update when the user signs in with their organization account.
// It encrypts the payload, binds a digest, requests a platform authorization URL, completes OAuth in the browser,
// exchanges the code for tokens, and saves credentials.
func (h *Handler) executeBrowserUpsert(ctx context.Context, inputs UpsertSecretsInputs, method string) error {
	if h.Credentials.AuthType == credentials.AuthTypeApiKey {
		return fmt.Errorf("this sign-in flow requires an interactive login; API keys are not supported")
	}
	orgID, err := h.Credentials.GetOrgID()
	if err != nil {
		return fmt.Errorf("organization information is missing from your session; sign in again or use owner-key-signing: %w", err)
	}

	ui.Dim("Using your account to authorize vault access for your organization...")

	encSecrets, err := h.EncryptSecretsForBrowserOrg(inputs, orgID)
	if err != nil {
		return fmt.Errorf("failed to encrypt secrets: %w", err)
	}
	requestID := uuid.New().String()

	var digest [32]byte

	switch method {
	case vaulttypes.MethodSecretsCreate:
		req := jsonrpc2.Request[vault.CreateSecretsRequest]{
			Version: jsonrpc2.JsonRpcVersion,
			ID:      requestID,
			Method:  method,
			Params: &vault.CreateSecretsRequest{
				RequestId:        requestID,
				EncryptedSecrets: encSecrets,
			},
		}
		digest, err = CalculateDigest(req)
		if err != nil {
			return fmt.Errorf("failed to calculate create digest: %w", err)
		}

	case vaulttypes.MethodSecretsUpdate:
		req := jsonrpc2.Request[vault.UpdateSecretsRequest]{
			Version: jsonrpc2.JsonRpcVersion,
			ID:      requestID,
			Method:  method,
			Params: &vault.UpdateSecretsRequest{
				RequestId:        requestID,
				EncryptedSecrets: encSecrets,
			},
		}
		digest, err = CalculateDigest(req)
		if err != nil {
			return fmt.Errorf("failed to calculate update digest: %w", err)
		}

	default:
		return fmt.Errorf("unsupported method %q (expected %q or %q)", method, vaulttypes.MethodSecretsCreate, vaulttypes.MethodSecretsUpdate)
	}

	perm, err := vaultPermissionForMethod(method)
	if err != nil {
		return err
	}

	verifier, challenge, err := oauth.GeneratePKCE()
	if err != nil {
		return err
	}

	gqlClient := graphqlclient.New(h.Credentials, h.EnvironmentSet, h.Log)
	gqlReq := graphql.NewRequest(createVaultAuthURLMutation)
	reqVars := map[string]any{
		"codeChallenge": challenge,
		"redirectUri":   constants.AuthRedirectURI,
		"requestDigest": digestHexString(digest),
		"permission":    perm,
	}
	// Optional: bind authorization to workflow owner when configured (omit if unset).
	if w := strings.TrimSpace(h.OwnerAddress); w != "" {
		reqVars["workflowOwnerAddress"] = w
	}
	gqlReq.Var("request", reqVars)

	var gqlResp struct {
		CreateVaultAuthorizationURL struct {
			URL string `json:"url"`
		} `json:"createVaultAuthorizationUrl"`
	}
	if err := gqlClient.Execute(ctx, gqlReq, &gqlResp); err != nil {
		return fmt.Errorf("could not complete the authorization request")
	}
	authURL := gqlResp.CreateVaultAuthorizationURL.URL
	if authURL == "" {
		return fmt.Errorf("could not complete the authorization request")
	}

	oauthIssuerBase, err := oauth.OAuthServerBaseFromAuthorizeURL(authURL)
	if err != nil {
		return fmt.Errorf("invalid authorization URL from server: %w", err)
	}
	platformState, _ := oauth.StateFromAuthorizeURL(authURL)
	oauthClientIDFromURL, _ := oauth.ClientIDFromAuthorizeURL(authURL)
	oauthClientIDForExchange := strings.TrimSpace(oauthClientIDFromURL)

	codeCh := make(chan string, 1)
	server, listener, err := oauth.NewCallbackHTTPServer(constants.AuthListenAddr, oauth.SecretsCallbackHandler(codeCh, platformState, h.Log))
	if err != nil {
		return fmt.Errorf("could not start local callback server: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			h.Log.Error().Err(err).Msg("secrets oauth callback server error")
		}
	}()

	ui.Dim("Opening your browser to complete sign-in...")
	if err := oauth.OpenBrowser(authURL, rt.GOOS); err != nil {
		ui.Warning("Could not open browser automatically")
		ui.Dim("Open this URL in your browser:")
	}
	ui.URL(authURL)
	ui.Line()
	ui.Dim("Waiting for authorization... (Press Ctrl+C to cancel)")

	var code string
	select {
	case code = <-codeCh:
	case <-time.After(500 * time.Second):
		return fmt.Errorf("timeout waiting for authorization")
	case <-ctx.Done():
		return ctx.Err()
	}

	ui.Dim("Saving credentials...")
	tokenSet, err := oauth.ExchangeAuthorizationCode(ctx, nil, h.EnvironmentSet, code, verifier, oauthClientIDForExchange, oauthIssuerBase)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}
	if err := credentials.SaveCredentials(tokenSet); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	ui.Success("Authorization completed successfully.")
	return nil
}

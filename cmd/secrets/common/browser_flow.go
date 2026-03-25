package common

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/machinebox/graphql"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
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
// It encrypts the payload, binds a digest, and completes the platform authorization request for this step.
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

	_, challenge, err := generatePKCES256()
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
	if gqlResp.CreateVaultAuthorizationURL.URL == "" {
		return fmt.Errorf("could not complete the authorization request")
	}

	ui.Success("Authorization completed successfully.")
	return nil
}

// generatePKCES256 builds the PKCE verifier and challenge used for secure authorization.
func generatePKCES256() (verifier string, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("pkce random: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

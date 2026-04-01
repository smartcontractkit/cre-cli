package common

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/internal/oauth"
)

func TestVaultPermissionForMethod(t *testing.T) {
	p, err := vaultPermissionForMethod(vaulttypes.MethodSecretsCreate)
	require.NoError(t, err)
	assert.Equal(t, "VAULT_PERMISSION_CREATE_SECRETS", p)

	p, err = vaultPermissionForMethod(vaulttypes.MethodSecretsUpdate)
	require.NoError(t, err)
	assert.Equal(t, "VAULT_PERMISSION_UPDATE_SECRETS", p)

	p, err = vaultPermissionForMethod(vaulttypes.MethodSecretsDelete)
	require.NoError(t, err)
	assert.Equal(t, "VAULT_PERMISSION_DELETE_SECRETS", p)

	p, err = vaultPermissionForMethod(vaulttypes.MethodSecretsList)
	require.NoError(t, err)
	assert.Equal(t, "VAULT_PERMISSION_LIST_SECRETS", p)

	_, err = vaultPermissionForMethod("vault/secrets/unknown")
	require.Error(t, err)
}

func TestDigestHexString(t *testing.T) {
	var d [32]byte
	copy(d[:], []byte{1, 2, 3})
	assert.Equal(t, "0x0102030000000000000000000000000000000000000000000000000000000000", digestHexString(d))
}

// TestBrowserFlowPKCE checks PKCE S256 (RFC 7636) used by the browser secrets authorization step.
func TestBrowserFlowPKCE(t *testing.T) {
	verifier, challenge, err := oauth.GeneratePKCE()
	require.NoError(t, err)
	require.NotEmpty(t, verifier)
	require.NotEmpty(t, challenge)

	sum := sha256.Sum256([]byte(verifier))
	decoded, err := base64.RawURLEncoding.DecodeString(challenge)
	require.NoError(t, err)
	assert.Equal(t, sum[:], decoded)
}

// postVaultGatewayWithBearer is the code path used after browser OAuth token exchange; it should stay aligned
// with owner-key gateway POST + ParseVaultGatewayResponse (minus allowlist retries).

func TestPostVaultGatewayWithBearer_CreateParsesResponse(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	var logBuf bytes.Buffer
	h := newTestHandler(&logBuf)
	h.Gw = &mockGatewayClient{
		post: func(gotBody []byte) ([]byte, int, error) {
			assert.Contains(t, string(gotBody), "jsonrpc")
			return encodeRPCBodyFromPayload(buildCreatePayloadProto(t)), http.StatusOK, nil
		},
	}

	err := h.postVaultGatewayWithBearer(vaulttypes.MethodSecretsCreate, []byte(`{"jsonrpc":"2.0","id":"1","method":"x"}`), "vault-jwt")
	w.Close()
	os.Stdout = oldStdout
	var out strings.Builder
	_, _ = io.Copy(&out, r)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Secret created")
}

func TestPostVaultGatewayWithBearer_ListParsesResponse(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	h := newTestHandler(nil)
	h.Gw = &mockGatewayClient{
		post: func([]byte) ([]byte, int, error) {
			return encodeRPCBodyFromPayload(buildListPayloadProtoSuccessWithItems(t)), http.StatusOK, nil
		},
	}

	err := h.postVaultGatewayWithBearer(vaulttypes.MethodSecretsList, []byte(`{}`), "t")
	w.Close()
	os.Stdout = oldStdout
	var out strings.Builder
	_, _ = io.Copy(&out, r)

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Secret identifier")
}

func TestPostVaultGatewayWithBearer_GatewayNon200(t *testing.T) {
	h, _, _ := newMockHandler(t)
	h.Gw = &mockGatewayClient{
		post: func([]byte) ([]byte, int, error) {
			return []byte(`denied`), http.StatusForbidden, nil
		},
	}

	err := h.postVaultGatewayWithBearer(vaulttypes.MethodSecretsDelete, []byte(`{}`), "t")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-200")
	assert.Contains(t, err.Error(), "403")
}

func TestPostVaultGatewayWithBearer_InvalidJSONRPC(t *testing.T) {
	h, _, _ := newMockHandler(t)
	h.Gw = &mockGatewayClient{
		post: func([]byte) ([]byte, int, error) {
			return []byte(`not-json`), http.StatusOK, nil
		},
	}

	err := h.postVaultGatewayWithBearer(vaulttypes.MethodSecretsUpdate, []byte(`{}`), "t")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

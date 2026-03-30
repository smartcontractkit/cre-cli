package common

import (
	"crypto/sha256"
	"encoding/base64"
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

package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePKCE_S256(t *testing.T) {
	verifier, challenge, err := GeneratePKCE()
	require.NoError(t, err)
	require.NotEmpty(t, verifier)
	require.NotEmpty(t, challenge)

	sum := sha256.Sum256([]byte(verifier))
	decoded, err := base64.RawURLEncoding.DecodeString(challenge)
	require.NoError(t, err)
	assert.Equal(t, sum[:], decoded)
}

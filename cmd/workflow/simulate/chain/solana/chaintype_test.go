package solana

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func newSolanaChainType() *SolanaChainType {
	lg := zerolog.Nop()
	return &SolanaChainType{log: &lg}
}

// keygenJSON renders key as the JSON array of numbers that `solana-keygen`
// writes. json.Marshal on a []byte would emit a base64 string instead, which
// is not the format users actually have on disk.
func keygenJSON(t *testing.T, key []byte) []byte {
	t.Helper()
	nums := make([]uint16, len(key))
	for i, b := range key {
		nums[i] = uint16(b)
	}
	b, err := json.Marshal(nums)
	require.NoError(t, err)
	return b
}

// writeKeygenFile writes key in the JSON byte-array form that `solana-keygen`
// produces and returns the file path.
func writeKeygenFile(t *testing.T, dir, name string, key []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, keygenJSON(t, key), 0o600))
	return path
}

func TestSolanaChainType_ResolveKey(t *testing.T) {
	key, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)

	dir := t.TempDir()
	keyfile := writeKeygenFile(t, dir, "id.json", key)
	shortFile := writeKeygenFile(t, dir, "short.json", key[:32])

	inline := keygenJSON(t, key)

	tests := []struct {
		name        string
		pk          string
		broadcast   bool
		wantErr     bool
		errContains string
	}{
		{
			name: "base58 keypair, non-broadcast",
			pk:   key.String(),
		},
		{
			name:      "base58 keypair, broadcast",
			pk:        key.String(),
			broadcast: true,
		},
		{
			name: "path to solana-keygen keyfile",
			pk:   keyfile,
		},
		{
			name:      "path to solana-keygen keyfile, broadcast",
			pk:        keyfile,
			broadcast: true,
		},
		{
			name: "inline keyfile contents",
			pk:   string(inline),
		},
		{
			name:        "empty",
			pk:          "",
			wantErr:     true,
			errContains: "CRE_SOLANA_PRIVATE_KEY is required",
		},
		{
			name:        "empty suggests the keyfile path",
			pk:          "   ",
			wantErr:     true,
			errContains: "~/.config/solana/id.json",
		},
		{
			name:        "garbage",
			pk:          "not-a-key",
			wantErr:     true,
			errContains: "must be a base58-encoded 64-byte keypair",
		},
		{
			name:        "nonexistent path",
			pk:          filepath.Join(dir, "missing.json"),
			wantErr:     true,
			errContains: "must be a base58-encoded 64-byte keypair",
		},
		{
			name:        "keyfile with 32 bytes instead of 64",
			pk:          shortFile,
			wantErr:     true,
			errContains: "invalid private key length 32",
		},
		{
			name:        "inline contents with 32 bytes instead of 64",
			pk:          "[1,2,3]",
			wantErr:     true,
			errContains: "invalid private key length 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := newSolanaChainType()
			s := &settings.Settings{User: settings.UserSettings{
				PrivateKeys: map[string]string{settings.Solana.Name: tt.pk},
			}}

			got, err := ct.ResolveKey(s, tt.broadcast)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			pk, ok := got.(solana.PrivateKey)
			require.True(t, ok, "expected solana.PrivateKey, got %T", got)
			assert.Equal(t, key.PublicKey(), pk.PublicKey())
		})
	}
}

// The ~ form is the one we tell users to configure, so cover it explicitly.
func TestSolanaChainType_ResolveKey_TildePath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	key, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)

	dir, err := os.MkdirTemp(home, "cre-solana-key-test-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	keyfile := writeKeygenFile(t, dir, "id.json", key)
	rel, err := filepath.Rel(home, keyfile)
	require.NoError(t, err)

	s := &settings.Settings{User: settings.UserSettings{
		PrivateKeys: map[string]string{settings.Solana.Name: filepath.Join("~", rel)},
	}}

	got, err := newSolanaChainType().ResolveKey(s, false)
	require.NoError(t, err)
	pk, ok := got.(solana.PrivateKey)
	require.True(t, ok, "expected solana.PrivateKey, got %T", got)
	assert.Equal(t, key.PublicKey(), pk.PublicKey())
}

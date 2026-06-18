package tenantctx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVaultPublicKeyFingerprint(t *testing.T) {
	fp1, err := VaultPublicKeyFingerprint("deadbeef")
	require.NoError(t, err)
	fp2, err := VaultPublicKeyFingerprint("0xDEADbeef")
	require.NoError(t, err)
	require.True(t, FingerprintsMatch(fp1, fp2))
	require.Len(t, fp1, 64)
}

func TestSaveAndLoadVaultKeyPin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, VaultKeyPinsFile)

	scope := VaultKeyPinScope{
		EnvName:             "staging",
		TenantID:            "tenant-1",
		CapRegChainSelector: 16015286601757825753,
		CapRegAddress:       "0xCapReg",
		VaultGatewayURL:     "https://gateway.example.com/",
	}

	require.NoError(t, saveVaultKeyPinToPath(path, scope, "abc123"))

	fp, ok, err := loadVaultKeyPinFromPath(path, scope)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "abc123", fp)

	otherScope := scope
	otherScope.TenantID = "other-tenant"
	_, ok, err = loadVaultKeyPinFromPath(path, otherScope)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestLoadVaultKeyPin_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, VaultKeyPinsFile)

	_, ok, err := loadVaultKeyPinFromPath(path, VaultKeyPinScope{EnvName: "staging"})
	require.NoError(t, err)
	require.False(t, ok)
}

func TestSaveVaultKeyPin_UsesCREDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	scope := VaultKeyPinScope{
		EnvName:             "PRODUCTION",
		TenantID:            "tenant-1",
		CapRegChainSelector: 1,
		CapRegAddress:       "0xabc",
		VaultGatewayURL:     "https://gateway.example.com/",
	}

	require.NoError(t, SaveVaultKeyPin(scope, "feedface"))

	fp, ok, err := LoadVaultKeyPin(VaultKeyPinScope{
		EnvName:             "production",
		TenantID:            "tenant-1",
		CapRegChainSelector: 1,
		CapRegAddress:       "0xabc",
		VaultGatewayURL:     "https://gateway.example.com/",
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "feedface", fp)

	pinPath, err := filepath.Abs(filepath.Join(home, ".cre", VaultKeyPinsFile))
	require.NoError(t, err)
	_, err = os.Stat(pinPath)
	require.NoError(t, err)
}

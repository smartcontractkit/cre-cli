package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

func pinTestHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func tofuTestTenantContext() *tenantctx.EnvironmentContext {
	return &tenantctx.EnvironmentContext{
		TenantID:        "tenant-1",
		VaultGatewayURL: "https://gateway.example.com/",
		CapabilitiesRegistry: &tenantctx.OnChainContract{
			ChainSelector: 16015286601757825753,
			Address:       "0x7f3191EaF73429177bAB3bAc5c36Ed2D5E39985f",
		},
	}
}

func TestVaultMasterPublicKeyHex_TOFUFirstPin(t *testing.T) {
	pinTestHome(t)

	h, _, _ := newMockHandler(t)
	h.TenantContext = tofuTestTenantContext()
	h.EnvironmentSet.EnvName = "staging"
	attachGatewayPublicKeyMock(t, h, vaultPublicKeyHex)
	attachMockVaultDONResolver(t, h, vaultPublicKeyHex)

	key, err := h.vaultMasterPublicKeyHex(h.execCtx)
	require.NoError(t, err)
	require.Equal(t, vaultPublicKeyHex, key)

	fp, ok, err := tenantctx.LoadVaultKeyPin(tenantctx.VaultKeyPinScope{
		EnvName:             "staging",
		TenantID:            "tenant-1",
		CapRegChainSelector: h.TenantContext.CapabilitiesRegistry.ChainSelector,
		CapRegAddress:       h.TenantContext.CapabilitiesRegistry.Address,
		VaultGatewayURL:     h.TenantContext.VaultGatewayURL,
	})
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEmpty(t, fp)
}

func TestVaultMasterPublicKeyHex_TOFUAbortsGatewayChangeWithoutOnChainUpdate(t *testing.T) {
	pinTestHome(t)

	h, _, _ := newMockHandler(t)
	h.TenantContext = tofuTestTenantContext()
	h.EnvironmentSet.EnvName = "staging"
	attachGatewayPublicKeyMock(t, h, vaultPublicKeyHex)
	attachMockVaultDONResolver(t, h, vaultPublicKeyHex)

	_, err := h.vaultMasterPublicKeyHex(h.execCtx)
	require.NoError(t, err)

	attachGatewayPublicKeyMock(t, h, "deadbeef")
	_, err = h.vaultMasterPublicKeyHex(h.execCtx)
	require.ErrorContains(t, err, "changed without a matching on-chain update")
}

func TestVaultMasterPublicKeyHex_TOFUAllowsOnChainRotation(t *testing.T) {
	pinTestHome(t)

	h, _, _ := newMockHandler(t)
	h.TenantContext = tofuTestTenantContext()
	h.EnvironmentSet.EnvName = "staging"
	attachGatewayPublicKeyMock(t, h, vaultPublicKeyHex)
	attachMockVaultDONResolver(t, h, vaultPublicKeyHex)

	_, err := h.vaultMasterPublicKeyHex(h.execCtx)
	require.NoError(t, err)

	rotatedKey := "cafebabe"
	attachGatewayPublicKeyMock(t, h, rotatedKey)
	attachMockVaultDONResolver(t, h, rotatedKey)

	key, err := h.vaultMasterPublicKeyHex(h.execCtx)
	require.NoError(t, err)
	require.Equal(t, rotatedKey, key)

	fp, ok, err := tenantctx.LoadVaultKeyPin(tenantctx.VaultKeyPinScope{
		EnvName:             "staging",
		TenantID:            "tenant-1",
		CapRegChainSelector: h.TenantContext.CapabilitiesRegistry.ChainSelector,
		CapRegAddress:       h.TenantContext.CapabilitiesRegistry.Address,
		VaultGatewayURL:     h.TenantContext.VaultGatewayURL,
	})
	require.NoError(t, err)
	require.True(t, ok)

	rotatedFP, err := tenantctx.VaultPublicKeyFingerprint(rotatedKey)
	require.NoError(t, err)
	require.True(t, tenantctx.FingerprintsMatch(fp, rotatedFP))
}

func TestVaultMasterPublicKeyHex_SkipsTOFUWhenValidationOptedOut(t *testing.T) {
	pinTestHome(t)

	h, _, _ := newMockHandler(t)
	h.TenantContext = tofuTestTenantContext()
	h.EnvironmentSet.EnvName = "staging"
	attachGatewayPublicKeyMock(t, h, vaultPublicKeyHex)
	attachMockVaultDONResolver(t, h, "deadbeef")
	h.skipVaultValidation = true

	key, err := h.vaultMasterPublicKeyHex(h.execCtx)
	require.NoError(t, err)
	require.Equal(t, vaultPublicKeyHex, key)

	_, ok, err := tenantctx.LoadVaultKeyPin(tenantctx.VaultKeyPinScope{EnvName: "staging"})
	require.NoError(t, err)
	require.False(t, ok)
}

package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

func strPtr(s string) *string { return &s }

func stagingEnvSet() *environments.EnvironmentSet {
	return &environments.EnvironmentSet{
		EnvName:                          "STAGING",
		WorkflowRegistryAddress:          "0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135",
		WorkflowRegistryChainName:        "ethereum-testnet-sepolia",
		WorkflowRegistryChainExplorerURL: "https://sepolia.etherscan.io",
	}
}

func sampleTenantCtx() *tenantctx.EnvironmentContext {
	return &tenantctx.EnvironmentContext{
		DefaultDonFamily: "zone-a",
		Registries: []*tenantctx.Registry{
			{
				ID:            "onchain:ethereum-testnet-sepolia",
				Label:         "ethereum-testnet-sepolia (0xaE55...1135)",
				Type:          "on-chain",
				ChainSelector: strPtr("16015286601757825753"),
				Address:       strPtr("0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135"),
			},
			{
				ID:    "private",
				Label: "Private (Chainlink-hosted)",
				Type:  "off-chain",
			},
		},
	}
}

func TestResolveRegistry_Empty_AddressAndChainFromEnvSet_NoDonWithoutTenantOrEnv(t *testing.T) {
	envSet := stagingEnvSet()
	resolved, err := ResolveRegistry("", nil, envSet)
	assert.NoError(t, err)

	onchain, ok := resolved.(*OnChainRegistry)
	assert.True(t, ok, "expected *OnChainRegistry, got %T", resolved)
	assert.Equal(t, envSet.WorkflowRegistryAddress, onchain.Address())
	assert.Equal(t, envSet.WorkflowRegistryChainName, onchain.ChainName())
	assert.Equal(t, "", onchain.DonFamily())
	assert.Equal(t, envSet.WorkflowRegistryChainExplorerURL, onchain.ExplorerURL())
}

func TestResolveRegistry_DefaultRegistry_UsesTenantDonFamily(t *testing.T) {
	envSet := stagingEnvSet()
	tenantCtx := &tenantctx.EnvironmentContext{DefaultDonFamily: "tenant-zone"}
	resolved, err := ResolveRegistry("", tenantCtx, envSet)
	assert.NoError(t, err)
	onchain, ok := resolved.(*OnChainRegistry)
	assert.True(t, ok, "expected *OnChainRegistry, got %T", resolved)
	assert.Equal(t, "tenant-zone", onchain.DonFamily())
}

func TestResolveRegistry_OnChainFromContext(t *testing.T) {
	resolved, err := ResolveRegistry("onchain:ethereum-testnet-sepolia", sampleTenantCtx(), stagingEnvSet())
	assert.NoError(t, err)

	onchain, ok := resolved.(*OnChainRegistry)
	assert.True(t, ok, "expected *OnChainRegistry, got %T", resolved)
	assert.Equal(t, "0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135", onchain.Address())
	assert.Equal(t, "ethereum-testnet-sepolia", onchain.ChainName())
	assert.Equal(t, "zone-a", onchain.DonFamily())
}

func TestResolveRegistry_NamedRegistry_EnvOverridesTenantDonFamily(t *testing.T) {
	envSet := stagingEnvSet()
	envSet.DonFamily = "from-env-var"
	tenantCtx := sampleTenantCtx()
	tenantCtx.DefaultDonFamily = "from-tenant"

	t.Run("on-chain named", func(t *testing.T) {
		resolved, err := ResolveRegistry("onchain:ethereum-testnet-sepolia", tenantCtx, envSet)
		assert.NoError(t, err)
		onchain, ok := resolved.(*OnChainRegistry)
		assert.True(t, ok, "expected *OnChainRegistry, got %T", resolved)
		assert.Equal(t, "from-env-var", onchain.DonFamily())
	})

	t.Run("private", func(t *testing.T) {
		resolved, err := ResolveRegistry("private", tenantCtx, envSet)
		assert.NoError(t, err)
		offchain, ok := resolved.(*OffChainRegistry)
		assert.True(t, ok, "expected *OffChainRegistry, got %T", resolved)
		assert.Equal(t, "from-env-var", offchain.DonFamily())
	})
}

func TestResolveRegistry_DefaultRegistry_EnvOverridesTenantDonFamily(t *testing.T) {
	envSet := stagingEnvSet()
	envSet.DonFamily = "from-env-var"
	tenantCtx := &tenantctx.EnvironmentContext{DefaultDonFamily: "tenant-zone"}
	resolved, err := ResolveRegistry("", tenantCtx, envSet)
	assert.NoError(t, err)
	onchain, ok := resolved.(*OnChainRegistry)
	assert.True(t, ok, "expected *OnChainRegistry, got %T", resolved)
	assert.Equal(t, "from-env-var", onchain.DonFamily())
}

func TestResolveRegistry_OffChainFromContext(t *testing.T) {
	resolved, err := ResolveRegistry("private", sampleTenantCtx(), stagingEnvSet())
	assert.NoError(t, err)

	offchain, ok := resolved.(*OffChainRegistry)
	assert.True(t, ok, "expected *OffChainRegistry, got %T", resolved)
	assert.Equal(t, "private", offchain.ID())
	assert.Equal(t, "zone-a", offchain.DonFamily())
	assert.Equal(t, RegistryTypeOffChain, resolved.Type())
}

func TestResolveRegistry_UnknownID(t *testing.T) {
	_, err := ResolveRegistry("does-not-exist", sampleTenantCtx(), stagingEnvSet())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in user context")
	assert.Contains(t, err.Error(), "onchain:ethereum-testnet-sepolia")
}

func TestResolveRegistry_NilTenantContextWithID(t *testing.T) {
	_, err := ResolveRegistry("private", nil, stagingEnvSet())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user context is not available")
}

func TestResolveRegistry_OnChainMissingAddress(t *testing.T) {
	ctx := &tenantctx.EnvironmentContext{
		DefaultDonFamily: "zone-a",
		Registries: []*tenantctx.Registry{
			{
				ID:            "onchain:no-addr",
				Type:          "on-chain",
				ChainSelector: strPtr("16015286601757825753"),
			},
		},
	}
	_, err := ResolveRegistry("onchain:no-addr", ctx, stagingEnvSet())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has no address")
}

func TestResolveRegistry_OnChainMissingChainSelector(t *testing.T) {
	ctx := &tenantctx.EnvironmentContext{
		DefaultDonFamily: "zone-a",
		Registries: []*tenantctx.Registry{
			{
				ID:      "onchain:no-chain",
				Type:    "on-chain",
				Address: strPtr("0x1234"),
			},
		},
	}
	_, err := ResolveRegistry("onchain:no-chain", ctx, stagingEnvSet())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "has no chain_selector")
}

func TestResolveRegistry_UnknownType(t *testing.T) {
	ctx := &tenantctx.EnvironmentContext{
		DefaultDonFamily: "zone-a",
		Registries: []*tenantctx.Registry{
			{
				ID:   "future-registry",
				Type: "unknown",
			},
		},
	}
	_, err := ResolveRegistry("future-registry", ctx, stagingEnvSet())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "future-registry")
	assert.Contains(t, err.Error(), "not supported by this CLI version")
}

func TestResolveRegistry_UnknownExcludedFromAvailable(t *testing.T) {
	ctx := &tenantctx.EnvironmentContext{
		DefaultDonFamily: "zone-a",
		Registries: []*tenantctx.Registry{
			{
				ID:   "private",
				Type: "off-chain",
			},
			{
				ID:   "future-registry",
				Type: "unknown",
			},
		},
	}
	_, err := ResolveRegistry("does-not-exist", ctx, stagingEnvSet())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in user context")
	assert.Contains(t, err.Error(), "private")
	assert.NotContains(t, err.Error(), "future-registry")
}

func TestParseRegistryType(t *testing.T) {
	valid := []struct {
		input string
		want  RegistryType
	}{
		{"on-chain", RegistryTypeOnChain},
		{"off-chain", RegistryTypeOffChain},
		{"unknown", RegistryTypeUnknown},
	}
	for _, tt := range valid {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseRegistryType(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	invalid := []string{"ON-CHAIN", "on_chain", "on-chian", ""}
	for _, input := range invalid {
		t.Run(input, func(t *testing.T) {
			_, err := ParseRegistryType(input)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "unrecognised registry type")
		})
	}
}

func TestInterfaceMethods(t *testing.T) {
	onchain := NewOnChainRegistry("oc-1", "0x1234", "sepolia", "zone-a", "https://etherscan.io")
	assert.Equal(t, RegistryTypeOnChain, onchain.Type())
	assert.Equal(t, "oc-1", onchain.ID())
	assert.Equal(t, "zone-a", onchain.DonFamily())

	offchain := NewOffChainRegistry("private", "zone-b")
	assert.Equal(t, RegistryTypeOffChain, offchain.Type())
	assert.Equal(t, "private", offchain.ID())
	assert.Equal(t, "zone-b", offchain.DonFamily())
}

func TestEffectiveDonFamily_TenantDefault(t *testing.T) {
	envSet := stagingEnvSet()
	tenantCtx := &tenantctx.EnvironmentContext{DefaultDonFamily: "tenant-zone"}
	assert.Equal(t, "tenant-zone", EffectiveDonFamily(envSet, tenantCtx))
}

func TestEffectiveDonFamily_EnvOverridesTenant(t *testing.T) {
	envSet := stagingEnvSet()
	envSet.DonFamily = "from-env"
	tenantCtx := &tenantctx.EnvironmentContext{DefaultDonFamily: "tenant-zone"}
	assert.Equal(t, "from-env", EffectiveDonFamily(envSet, tenantCtx))
}

func TestEffectiveDonFamily_EmptyWhenNeitherSet(t *testing.T) {
	envSet := stagingEnvSet()
	assert.Equal(t, "", EffectiveDonFamily(envSet, nil))
	assert.Equal(t, "", EffectiveDonFamily(envSet, &tenantctx.EnvironmentContext{}))
}

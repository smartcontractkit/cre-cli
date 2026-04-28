package settings

import (
	"strings"
	"testing"

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
		DonFamily:                        "zone-a",
	}
}

func prodEnvSet() *environments.EnvironmentSet {
	return &environments.EnvironmentSet{
		EnvName:                          "PRODUCTION",
		WorkflowRegistryAddress:          "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5",
		WorkflowRegistryChainName:        "ethereum-mainnet",
		WorkflowRegistryChainExplorerURL: "https://etherscan.io",
		DonFamily:                        "zone-a",
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

func TestResolveRegistry_EmptyFallsBackToEnvSet(t *testing.T) {
	envSet := stagingEnvSet()
	resolved, err := ResolveRegistry("", nil, envSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	onchain, ok := resolved.(*OnChainRegistry)
	if !ok {
		t.Fatalf("expected *OnChainRegistry, got %T", resolved)
	}
	if onchain.Address() != envSet.WorkflowRegistryAddress {
		t.Errorf("expected address %s, got %s", envSet.WorkflowRegistryAddress, onchain.Address())
	}
	if onchain.ChainName() != envSet.WorkflowRegistryChainName {
		t.Errorf("expected chain %s, got %s", envSet.WorkflowRegistryChainName, onchain.ChainName())
	}
	if onchain.DonFamily() != envSet.DonFamily {
		t.Errorf("expected don %s, got %s", envSet.DonFamily, onchain.DonFamily())
	}
	if onchain.ExplorerURL() != envSet.WorkflowRegistryChainExplorerURL {
		t.Errorf("expected explorer %s, got %s", envSet.WorkflowRegistryChainExplorerURL, onchain.ExplorerURL())
	}
}

func TestResolveRegistry_OnChainFromContext(t *testing.T) {
	resolved, err := ResolveRegistry("onchain:ethereum-testnet-sepolia", sampleTenantCtx(), stagingEnvSet())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	onchain, ok := resolved.(*OnChainRegistry)
	if !ok {
		t.Fatalf("expected *OnChainRegistry, got %T", resolved)
	}
	if onchain.Address() != "0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135" {
		t.Errorf("unexpected address: %s", onchain.Address())
	}
	if onchain.ChainName() != "ethereum-testnet-sepolia" {
		t.Errorf("unexpected chain name: %s", onchain.ChainName())
	}
	if onchain.DonFamily() != "zone-a" {
		t.Errorf("unexpected don family: %s", onchain.DonFamily())
	}
}

func TestResolveRegistry_OffChainFromContext(t *testing.T) {
	resolved, err := ResolveRegistry("private", sampleTenantCtx(), stagingEnvSet())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	offchain, ok := resolved.(*OffChainRegistry)
	if !ok {
		t.Fatalf("expected *OffChainRegistry, got %T", resolved)
	}
	if offchain.ID() != "private" {
		t.Errorf("expected ID %q, got %q", "private", offchain.ID())
	}
	if offchain.DonFamily() != "zone-a" {
		t.Errorf("unexpected don family: %s", offchain.DonFamily())
	}
	if resolved.Type() != RegistryTypeOffChain {
		t.Errorf("expected type %s, got %s", RegistryTypeOffChain, resolved.Type())
	}
}

func TestResolveRegistry_UnknownID(t *testing.T) {
	_, err := ResolveRegistry("does-not-exist", sampleTenantCtx(), stagingEnvSet())
	if err == nil {
		t.Fatal("expected error for unknown registry ID")
	}
	if !strings.Contains(err.Error(), "not found in user context") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "onchain:ethereum-testnet-sepolia") {
		t.Errorf("error should list available IDs: %v", err)
	}
}

func TestResolveRegistry_NilTenantContextWithID(t *testing.T) {
	_, err := ResolveRegistry("private", nil, stagingEnvSet())
	if err == nil {
		t.Fatal("expected error when TenantContext is nil with a registry ID set")
	}
	if !strings.Contains(err.Error(), "user context is not available") {
		t.Errorf("unexpected error: %v", err)
	}
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
	if err == nil {
		t.Fatal("expected error for on-chain registry without address")
	}
	if !strings.Contains(err.Error(), "has no address") {
		t.Errorf("unexpected error: %v", err)
	}
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
	if err == nil {
		t.Fatal("expected error for on-chain registry without chain selector")
	}
	if !strings.Contains(err.Error(), "has no chain_selector") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseRegistryType(t *testing.T) {
	tests := []struct {
		input string
		want  RegistryType
	}{
		{"on-chain", RegistryTypeOnChain},
		{"off-chain", RegistryTypeOffChain},
		{"ON-CHAIN", RegistryTypeOnChain},
		{"OFF-CHAIN", RegistryTypeOffChain},
		{"off_chain", RegistryTypeOffChain},
		{"unknown", RegistryTypeOnChain},
	}
	for _, tt := range tests {
		if got := ParseRegistryType(tt.input); got != tt.want {
			t.Errorf("ParseRegistryType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestInterfaceMethods(t *testing.T) {
	onchain := NewOnChainRegistry("oc-1", "0x1234", "sepolia", "zone-a", "https://etherscan.io")
	if onchain.Type() != RegistryTypeOnChain {
		t.Errorf("expected on-chain type")
	}
	if onchain.ID() != "oc-1" {
		t.Errorf("expected ID oc-1, got %s", onchain.ID())
	}
	if onchain.DonFamily() != "zone-a" {
		t.Errorf("expected don zone-a, got %s", onchain.DonFamily())
	}

	offchain := NewOffChainRegistry("private", "zone-b")
	if offchain.Type() != RegistryTypeOffChain {
		t.Errorf("expected off-chain type")
	}
	if offchain.ID() != "private" {
		t.Errorf("expected ID private, got %s", offchain.ID())
	}
	if offchain.DonFamily() != "zone-b" {
		t.Errorf("expected don zone-b, got %s", offchain.DonFamily())
	}
}

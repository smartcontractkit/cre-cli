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
	if resolved.Type != "on-chain" {
		t.Errorf("expected on-chain, got %s", resolved.Type)
	}
	if resolved.Address != envSet.WorkflowRegistryAddress {
		t.Errorf("expected address %s, got %s", envSet.WorkflowRegistryAddress, resolved.Address)
	}
	if resolved.ChainName != envSet.WorkflowRegistryChainName {
		t.Errorf("expected chain %s, got %s", envSet.WorkflowRegistryChainName, resolved.ChainName)
	}
	if resolved.DonFamily != envSet.DonFamily {
		t.Errorf("expected don %s, got %s", envSet.DonFamily, resolved.DonFamily)
	}
	if resolved.ExplorerURL != envSet.WorkflowRegistryChainExplorerURL {
		t.Errorf("expected explorer %s, got %s", envSet.WorkflowRegistryChainExplorerURL, resolved.ExplorerURL)
	}
}

func TestResolveRegistry_OnChainFromContext(t *testing.T) {
	resolved, err := ResolveRegistry("onchain:ethereum-testnet-sepolia", sampleTenantCtx(), stagingEnvSet())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "on-chain" {
		t.Errorf("expected on-chain, got %s", resolved.Type)
	}
	if resolved.Address != "0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135" {
		t.Errorf("unexpected address: %s", resolved.Address)
	}
	if resolved.ChainName != "ethereum-testnet-sepolia" {
		t.Errorf("unexpected chain name: %s", resolved.ChainName)
	}
	if resolved.DonFamily != "zone-a" {
		t.Errorf("unexpected don family: %s", resolved.DonFamily)
	}
}

func TestResolveRegistry_OffChainFromContext(t *testing.T) {
	resolved, err := ResolveRegistry("private", sampleTenantCtx(), stagingEnvSet())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Type != "off-chain" {
		t.Errorf("expected off-chain, got %s", resolved.Type)
	}
	if resolved.Address != "" {
		t.Errorf("expected empty address for off-chain, got %s", resolved.Address)
	}
	if resolved.ChainName != "" {
		t.Errorf("expected empty chain for off-chain, got %s", resolved.ChainName)
	}
	if resolved.DonFamily != "zone-a" {
		t.Errorf("unexpected don family: %s", resolved.DonFamily)
	}
}

func TestResolveRegistry_UnknownID(t *testing.T) {
	_, err := ResolveRegistry("does-not-exist", sampleTenantCtx(), stagingEnvSet())
	if err == nil {
		t.Fatal("expected error for unknown registry ID")
	}
	if !strings.Contains(err.Error(), "not found in context.yaml") {
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

func TestResolveRegistry_OffChainBlockedInProduction(t *testing.T) {
	_, err := ResolveRegistry("private", sampleTenantCtx(), prodEnvSet())
	if err == nil {
		t.Fatal("expected error for off-chain in production")
	}
	if !strings.Contains(err.Error(), "not yet supported in production") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveRegistry_OffChainBlockedWhenEnvEmpty(t *testing.T) {
	envSet := stagingEnvSet()
	envSet.EnvName = ""
	_, err := ResolveRegistry("private", sampleTenantCtx(), envSet)
	if err == nil {
		t.Fatal("expected error for off-chain when env name is empty (defaults to production)")
	}
	if !strings.Contains(err.Error(), "not yet supported in production") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRequireOnChainRegistry(t *testing.T) {
	onChain := &ResolvedRegistry{ID: "onchain:ethereum-testnet-sepolia", Type: "on-chain"}
	if err := onChain.RequireOnChainRegistry("deploy"); err != nil {
		t.Errorf("on-chain should pass: %v", err)
	}

	offChain := &ResolvedRegistry{ID: "private", Type: "off-chain"}
	if err := offChain.RequireOnChainRegistry("deploy"); err == nil {
		t.Error("off-chain should be rejected")
	}
}

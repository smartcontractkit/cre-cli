package workflowrender

import (
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

func sp(s string) *string { return &s }

func TestRowMatchesRegistry_DirectIDMatch(t *testing.T) {
	reg := &tenantctx.Registry{ID: "private", Type: "off-chain"}
	all := []*tenantctx.Registry{reg}

	if !rowMatchesRegistry("private", reg, all) {
		t.Error("expected direct ID match to return true")
	}
	if rowMatchesRegistry("other", reg, all) {
		t.Error("expected non-matching ID to return false")
	}
}

func TestRowMatchesRegistry_ContractSource_MatchesByAddress(t *testing.T) {
	chainSel := "12345678901234567890"
	addr := "0xdeadbeef00000000000000000000000000000000"
	reg := &tenantctx.Registry{
		ID:            "onchain:testnet",
		ChainSelector: sp(chainSel),
		Address:       sp(addr),
	}
	all := []*tenantctx.Registry{reg, {ID: "private", Type: "off-chain"}}

	source := "contract:" + chainSel + ":" + addr
	if !rowMatchesRegistry(source, reg, all) {
		t.Errorf("expected contract source %q to match registry with address %q", source, addr)
	}
}

func TestRowMatchesRegistry_ContractSource_AddressCaseInsensitive(t *testing.T) {
	chainSel := "99999999999999999999"
	addr := "0xABCDEF0000000000000000000000000000000000"
	reg := &tenantctx.Registry{
		ID:            "onchain:case-test",
		ChainSelector: sp(chainSel),
		Address:       sp(addr),
	}
	all := []*tenantctx.Registry{reg}

	lowerSource := "contract:" + chainSel + ":0xabcdef0000000000000000000000000000000000"
	if !rowMatchesRegistry(lowerSource, reg, all) {
		t.Errorf("expected case-insensitive address match for source %q", lowerSource)
	}
}

func TestRowMatchesRegistry_ContractSource_WrongChainSelector(t *testing.T) {
	addr := "0xdeadbeef00000000000000000000000000000000"
	reg := &tenantctx.Registry{
		ID:            "onchain:testnet",
		ChainSelector: sp("11111111111111111111"),
		Address:       sp(addr),
	}
	all := []*tenantctx.Registry{reg}

	source := "contract:99999999999999999999:" + addr
	if rowMatchesRegistry(source, reg, all) {
		t.Errorf("expected wrong chain selector to NOT match")
	}
}

func TestRowMatchesRegistry_ContractSource_NoMatchForOffChainRegistry(t *testing.T) {
	offChain := &tenantctx.Registry{ID: "private", Type: "off-chain"}
	all := []*tenantctx.Registry{offChain}

	source := "contract:12345678901234567890:0xdeadbeef00000000000000000000000000000000"
	if rowMatchesRegistry(source, offChain, all) {
		t.Error("expected contract source to NOT match an off-chain registry")
	}
}

func TestRowMatchesRegistry_GrpcSource_SingleEligibleRegistry(t *testing.T) {
	private := &tenantctx.Registry{ID: "private", Type: "off-chain"}
	all := []*tenantctx.Registry{
		{ID: "onchain:testnet", ChainSelector: sp("12345678901234567890"), Address: sp("0xaaaa")},
		private,
	}

	if !rowMatchesRegistry("grpc:some-endpoint:v1", private, all) {
		t.Error("expected grpc source to match the single eligible off-chain registry")
	}
}

func TestRowMatchesRegistry_GrpcSource_NoMatchForOnChainRegistry(t *testing.T) {
	onchain := &tenantctx.Registry{
		ID:            "onchain:testnet",
		ChainSelector: sp("12345678901234567890"),
		Address:       sp("0xdeadbeef00000000000000000000000000000000"),
	}
	all := []*tenantctx.Registry{onchain}

	if rowMatchesRegistry("grpc:some-endpoint:v1", onchain, all) {
		t.Error("expected grpc source to NOT match an on-chain registry")
	}
}

func TestRowMatchesRegistry_GrpcSource_MatchByIDSubstring(t *testing.T) {
	reg := &tenantctx.Registry{ID: "private", Type: "off-chain"}
	other := &tenantctx.Registry{ID: "staging", Type: "off-chain"}
	all := []*tenantctx.Registry{reg, other}

	// Contains "private" in the source — should resolve to reg.
	if !rowMatchesRegistry("grpc:private-endpoint:v1", reg, all) {
		t.Error("expected grpc source containing registry ID substring to match that registry")
	}
	if rowMatchesRegistry("grpc:private-endpoint:v1", other, all) {
		t.Error("expected grpc source to NOT match the non-matching registry")
	}
}

func TestRowMatchesRegistry_UnknownSourceFormat(t *testing.T) {
	reg := &tenantctx.Registry{ID: "private", Type: "off-chain"}
	all := []*tenantctx.Registry{reg}

	if rowMatchesRegistry("unknown:format:xyz", reg, all) {
		t.Error("expected unknown source format to NOT match any registry")
	}
}

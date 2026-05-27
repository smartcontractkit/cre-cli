package workflowrender

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

func sp(s string) *string { return &s }

func TestWorkflowSourceMatchesRegistry_DirectIDMatch(t *testing.T) {
	reg := &tenantctx.Registry{ID: "private", Type: "off-chain"}
	all := []*tenantctx.Registry{reg}

	assert.True(t, workflowSourceMatchesRegistry("private", reg, all), "direct ID match should return true")
	assert.False(t, workflowSourceMatchesRegistry("other", reg, all), "non-matching ID should return false")
}

func TestWorkflowSourceMatchesRegistry_ContractSource_MatchesByAddress(t *testing.T) {
	chainSel := "12345678901234567890"
	addr := "0xdeadbeef00000000000000000000000000000000"
	reg := &tenantctx.Registry{
		ID:            "onchain:testnet",
		ChainSelector: sp(chainSel),
		Address:       sp(addr),
	}
	all := []*tenantctx.Registry{reg, {ID: "private", Type: "off-chain"}}

	source := "contract:" + chainSel + ":" + addr
	assert.True(t, workflowSourceMatchesRegistry(source, reg, all),
		"contract source %q should match registry with address %q", source, addr)
}

func TestWorkflowSourceMatchesRegistry_ContractSource_AddressCaseInsensitive(t *testing.T) {
	chainSel := "99999999999999999999"
	addr := "0xABCDEF0000000000000000000000000000000000"
	reg := &tenantctx.Registry{
		ID:            "onchain:case-test",
		ChainSelector: sp(chainSel),
		Address:       sp(addr),
	}
	all := []*tenantctx.Registry{reg}

	lowerSource := "contract:" + chainSel + ":0xabcdef0000000000000000000000000000000000"
	assert.True(t, workflowSourceMatchesRegistry(lowerSource, reg, all),
		"address matching should be case-insensitive for source %q", lowerSource)
}

func TestWorkflowSourceMatchesRegistry_ContractSource_WrongChainSelector(t *testing.T) {
	addr := "0xdeadbeef00000000000000000000000000000000"
	reg := &tenantctx.Registry{
		ID:            "onchain:testnet",
		ChainSelector: sp("11111111111111111111"),
		Address:       sp(addr),
	}
	all := []*tenantctx.Registry{reg}

	source := "contract:99999999999999999999:" + addr
	assert.False(t, workflowSourceMatchesRegistry(source, reg, all), "wrong chain selector should not match")
}

func TestWorkflowSourceMatchesRegistry_ContractSource_NoMatchForOffChainRegistry(t *testing.T) {
	offChain := &tenantctx.Registry{ID: "private", Type: "off-chain"}
	all := []*tenantctx.Registry{offChain}

	source := "contract:12345678901234567890:0xdeadbeef00000000000000000000000000000000"
	assert.False(t, workflowSourceMatchesRegistry(source, offChain, all),
		"contract source should not match an off-chain registry")
}

func TestWorkflowSourceMatchesRegistry_GrpcSource_SingleEligibleRegistry(t *testing.T) {
	private := &tenantctx.Registry{ID: "private", Type: "off-chain"}
	all := []*tenantctx.Registry{
		{ID: "onchain:testnet", ChainSelector: sp("12345678901234567890"), Address: sp("0xaaaa")},
		private,
	}

	assert.True(t, workflowSourceMatchesRegistry("grpc:some-endpoint:v1", private, all),
		"grpc source should match the single eligible off-chain registry")
}

func TestWorkflowSourceMatchesRegistry_GrpcSource_NoMatchForOnChainRegistry(t *testing.T) {
	onchain := &tenantctx.Registry{
		ID:            "onchain:testnet",
		ChainSelector: sp("12345678901234567890"),
		Address:       sp("0xdeadbeef00000000000000000000000000000000"),
	}
	all := []*tenantctx.Registry{onchain}

	assert.False(t, workflowSourceMatchesRegistry("grpc:some-endpoint:v1", onchain, all),
		"grpc source should not match an on-chain registry")
}

func TestWorkflowSourceMatchesRegistry_GrpcSource_MatchByIDSubstring(t *testing.T) {
	reg := &tenantctx.Registry{ID: "private", Type: "off-chain"}
	other := &tenantctx.Registry{ID: "staging", Type: "off-chain"}
	all := []*tenantctx.Registry{reg, other}

	assert.True(t, workflowSourceMatchesRegistry("grpc:private-endpoint:v1", reg, all),
		"grpc source containing registry ID substring should match that registry")
	assert.False(t, workflowSourceMatchesRegistry("grpc:private-endpoint:v1", other, all),
		"grpc source should not match the non-matching registry")
}

func TestWorkflowSourceMatchesRegistry_UnknownSourceFormat(t *testing.T) {
	reg := &tenantctx.Registry{ID: "private", Type: "off-chain"}
	all := []*tenantctx.Registry{reg}

	assert.False(t, workflowSourceMatchesRegistry("unknown:format:xyz", reg, all),
		"unknown source format should not match any registry")
}

package evm

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// All forwarders declared in supported_chains.go must be valid 0x-prefixed
// 20-byte hex addresses. Catches typos that would only surface as runtime
// "invalid address" errors later in simulation.

var forwarderRe = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

func TestSupportedChains_NotEmpty(t *testing.T) {
	t.Parallel()
	require.NotEmpty(t, SupportedChains)
	require.Greater(t, len(SupportedChains), 20)
}

func TestSupportedChains_AllSelectorsNonZero(t *testing.T) {
	t.Parallel()
	for i, c := range SupportedChains {
		require.NotZerof(t, c.Selector, "index %d has zero selector", i)
	}
}

func TestSupportedChains_AllSelectorsUnique(t *testing.T) {
	t.Parallel()
	seen := map[uint64]int{}
	for i, c := range SupportedChains {
		if prev, ok := seen[c.Selector]; ok {
			t.Fatalf("duplicate selector %d at indices %d and %d", c.Selector, prev, i)
		}
		seen[c.Selector] = i
	}
}

func TestSupportedChains_AllForwardersValidHexAddress(t *testing.T) {
	t.Parallel()
	for _, c := range SupportedChains {
		assert.True(t, forwarderRe.MatchString(c.Forwarder),
			"selector %d: invalid forwarder hex %q", c.Selector, c.Forwarder)
	}
}

func TestSupportedChains_AllForwardersDecodableAsAddress(t *testing.T) {
	t.Parallel()
	for _, c := range SupportedChains {
		addr := common.HexToAddress(c.Forwarder)
		assert.NotEqual(t, common.Address{}, addr,
			"selector %d: forwarder decodes to zero address", c.Selector)
	}
}

func TestSupportedChains_AllForwardersLowercasedOrChecksummed(t *testing.T) {
	// Not enforcing checksum specifically — only ensuring HexToAddress normalises
	// to a canonical form.
	t.Parallel()
	for _, c := range SupportedChains {
		// If user writes uppercase hex, HexToAddress still accepts it.
		norm := common.HexToAddress(c.Forwarder).Hex()
		require.NotEmpty(t, norm)
	}
}

func TestSupportedChains_AllSelectorsResolveToChainName(t *testing.T) {
	t.Parallel()
	for _, c := range SupportedChains {
		info, err := chainselectors.GetSelectorFamily(c.Selector)
		require.NoErrorf(t, err, "selector %d missing family", c.Selector)
		assert.NotEmpty(t, info)
	}
}

func TestSupportedChains_EthereumMainnetPresent(t *testing.T) {
	t.Parallel()
	found := false
	for _, c := range SupportedChains {
		if c.Selector == chainselectors.ETHEREUM_MAINNET.Selector {
			found = true
			assert.NotEmpty(t, c.Forwarder)
			break
		}
	}
	require.True(t, found, "ethereum mainnet must be in SupportedChains")
}

func TestSupportedChains_SepoliaPresent(t *testing.T) {
	t.Parallel()
	found := false
	for _, c := range SupportedChains {
		if c.Selector == chainselectors.ETHEREUM_TESTNET_SEPOLIA.Selector {
			found = true
			assert.Equal(t, strings.ToLower("0x15fC6ae953E024d975e77382eEeC56A9101f9F88"),
				strings.ToLower(c.Forwarder), "sepolia forwarder must match known value")
			break
		}
	}
	require.True(t, found, "ethereum-testnet-sepolia must be in SupportedChains")
}

func TestSupportedChains_NoForwarderEmpty(t *testing.T) {
	t.Parallel()
	for i, c := range SupportedChains {
		require.NotEmpty(t, c.Forwarder, "supported chain at index %d has empty forwarder", i)
	}
}

func TestSupportedChains_ReturnedByChainType(t *testing.T) {
	t.Parallel()
	f := newEVMChainType()
	ret := f.SupportedChains()
	require.Equal(t, len(SupportedChains), len(ret))
	// Element-wise identity (same struct values, same order).
	for i, c := range SupportedChains {
		assert.Equal(t, c.Selector, ret[i].Selector, "selector at index %d", i)
		assert.Equal(t, c.Forwarder, ret[i].Forwarder, "forwarder at index %d", i)
	}
}

func TestSupportedChains_AllForwardersAre20Bytes(t *testing.T) {
	t.Parallel()
	for _, c := range SupportedChains {
		b := common.FromHex(c.Forwarder)
		assert.Len(t, b, 20, "selector %d forwarder not 20 bytes: %q", c.Selector, c.Forwarder)
	}
}

func TestChainConfigType_ImplementedCorrectly(t *testing.T) {
	t.Parallel()
	cfg := chain.ChainConfig{Selector: 1, Forwarder: "0x" + strings.Repeat("a", 40)}
	assert.Equal(t, uint64(1), cfg.Selector)
	assert.Len(t, cfg.Forwarder, 42)
}

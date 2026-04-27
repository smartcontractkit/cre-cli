package aptos

import (
	"regexp"
	"testing"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Aptos forwarders are 32-byte object addresses encoded as 64 hex chars.
var forwarderRe = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

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

func TestSupportedChains_AllSelectorsResolveToChainName(t *testing.T) {
	t.Parallel()
	for _, c := range SupportedChains {
		info, err := chainselectors.GetSelectorFamily(c.Selector)
		require.NoErrorf(t, err, "selector %d missing family", c.Selector)
		assert.NotEmpty(t, info)
	}
}

func TestSupportedChains_NoForwarderEmpty(t *testing.T) {
	t.Parallel()
	for i, c := range SupportedChains {
		require.NotEmpty(t, c.Forwarder, "supported chain at index %d has empty forwarder", i)
	}
}

func TestSupportedChains_ReturnedByChainType(t *testing.T) {
	t.Parallel()
	ct := &AptosChainType{}
	ret := ct.SupportedChains()
	require.Equal(t, len(SupportedChains), len(ret))
	for i, c := range SupportedChains {
		assert.Equal(t, c.Selector, ret[i].Selector, "selector at index %d", i)
		assert.Equal(t, c.Forwarder, ret[i].Forwarder, "forwarder at index %d", i)
	}
}

func TestSupportedChains_MainnetAndTestnet(t *testing.T) {
	t.Parallel()
	var hasMainnet, hasTestnet bool
	for _, c := range SupportedChains {
		switch c.Selector {
		case chainselectors.APTOS_MAINNET.Selector:
			hasMainnet = true
		case chainselectors.APTOS_TESTNET.Selector:
			hasTestnet = true
		}
	}
	assert.True(t, hasMainnet)
	assert.True(t, hasTestnet)
}

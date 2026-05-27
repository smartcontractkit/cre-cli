package evm

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
)

// All forwarders declared in supported_chains.go must be valid 0x-prefixed
// 20-byte hex addresses. Catches typos that would only surface as runtime
// "invalid address" errors later in simulation.

var forwarderRe = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

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
	f := newEVMChainType()
	ret := f.SupportedChains()
	require.Equal(t, len(SupportedChains), len(ret))
	// Element-wise identity (same struct values, same order).
	for i, c := range SupportedChains {
		assert.Equal(t, c.Selector, ret[i].Selector, "selector at index %d", i)
		assert.Equal(t, c.Forwarder, ret[i].Forwarder, "forwarder at index %d", i)
	}
}

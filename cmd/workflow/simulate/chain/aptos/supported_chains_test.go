package aptos

import (
	"testing"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/assert"
)

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

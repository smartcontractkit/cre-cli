package aptos

import (
	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// placeholderForwarder is used until canonical platform_mock addresses are
// published per network. Users override via experimental-chains config.
const placeholderForwarder = "0x0000000000000000000000000000000000000000000000000000000000000000"

// SupportedChains lists Aptos networks cre-cli simulate can target.
var SupportedChains = []chain.ChainConfig{
	{Selector: chainselectors.APTOS_MAINNET.Selector, Forwarder: placeholderForwarder},
	{Selector: chainselectors.APTOS_TESTNET.Selector, Forwarder: placeholderForwarder},
}

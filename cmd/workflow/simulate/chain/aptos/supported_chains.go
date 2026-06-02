package aptos

import (
	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// platform_mock MockForwarder object addresses, deployed per network
// (chainlink-aptos contracts/platform_mock, published via scripts/publish_mock.sh).
// Users can still override via experimental-chains config (chain-type: aptos).
const (
	mainnetForwarder = "0xbe43e1d5f2432c31b0026661a4eab8969531c61d449b45a83d199a560314c6c4"
	testnetForwarder = "0xef1c83c5c5c05f6604d17576a49570c28b49e4955d96d32e130c4ce5a1bc51b8"
)

// SupportedChains lists Aptos networks cre-cli simulate can target.
var SupportedChains = []chain.ChainConfig{
	{Selector: chainselectors.APTOS_MAINNET.Selector, Forwarder: mainnetForwarder},
	{Selector: chainselectors.APTOS_TESTNET.Selector, Forwarder: testnetForwarder},
}

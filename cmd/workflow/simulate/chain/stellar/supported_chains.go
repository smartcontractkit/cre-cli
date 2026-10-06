package stellar

import (
	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// Mock forwarder contracts (chainlink-stellar/contracts/cre/mock_forwarder),
// deployed per network. The mock accepts reports without DON signatures, so
// simulated reports reach the receiver; receivers see it as on_report's
// `sender`. Users can override per target with an experimental-chains entry
// (chain-type: stellar, forwarder: C…).
const (
	testnetMockForwarder = "CC5OVNNU32FQAWHDWIGFUWYRLMPVERESM52S4IYC3MMETI2YDX3N674X"
	mainnetMockForwarder = "CBBNYOUOFBQZDJFN4GVFODGOPDY6DRRWZBWGXFGJQZBRFXSFAQPX3L6T"
)

// SupportedChains lists Stellar networks cre-cli simulate can target.
var SupportedChains = []chain.ChainConfig{
	{Selector: chainselectors.STELLAR_TESTNET.Selector, Forwarder: testnetMockForwarder},
	{Selector: chainselectors.STELLAR_MAINNET.Selector, Forwarder: mainnetMockForwarder},
}

// networkPassphrase returns the network passphrase for a Stellar selector, or
// "" when the selector is unknown to chain-selectors.
func networkPassphrase(selector uint64) string {
	for _, c := range chainselectors.StellarALL {
		if c.Selector == selector {
			return c.Passphrase
		}
	}
	return ""
}

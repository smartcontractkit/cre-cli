package solana

import (
	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// mock_forwarder program + state account addresses, deployed per network from
// chainlink-solana/contracts/programs/mock-forwarder. Users can override via
// experimental-chains config (chain-type: solana).
const (
	devnetMockForwarderProgramID  = "7kuEAA3mSC1Tz8gQjnvH7bKFda9xSPRRin9SZbH49cNK"
	mainnetMockForwarderProgramID = "7kuEAA3mSC1Tz8gQjnvH7bKFda9xSPRRin9SZbH49cNK"
)

// SupportedChains lists Solana networks cre-cli simulate can target.
// Forwarder field stores the mock_forwarder program ID. The per-selector
// forwarder *state account* is kept in forwarderStateAccounts because
// chain.ChainConfig only carries one address.
var SupportedChains = []chain.ChainConfig{
	{Selector: chainselectors.SOLANA_DEVNET.Selector, Forwarder: devnetMockForwarderProgramID},
	{Selector: chainselectors.SOLANA_MAINNET.Selector, Forwarder: mainnetMockForwarderProgramID},
}

// forwarderStateAccounts maps chain selector → mock_forwarder state account.
// Required because the on-chain `report` instruction needs both program ID
// (resolved via chain.ChainConfig.Forwarder) and state account (here).
var forwarderStateAccounts = map[uint64]string{
	chainselectors.SOLANA_DEVNET.Selector:  "5Tipz3yhTBdVsDbaBxZkrp7Gjf3brGq5SKkxReefPMP7",
	chainselectors.SOLANA_MAINNET.Selector: "jhCjuD4Z3V7HeSUChMRpkRwpw6B9yC63mxDMv8SdLNX",
}

package evm

import (
	"sort"

	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// SupportedChainNames returns the human-readable names of all supported EVM chains,
// sorted alphabetically.
func SupportedChainNames() []string {
	var names []string
	for _, c := range SupportedChains {
		name, err := settings.GetChainNameByChainSelector(c.Selector)
		if err != nil {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SupportedChains is the canonical list of EVM chains supported for simulation.
var SupportedChains = []chain.ChainConfig{
	// Ethereum
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA.Selector, Forwarder: "0x15fC6ae953E024d975e77382eEeC56A9101f9F88"},
	{Selector: chainselectors.ETHEREUM_MAINNET.Selector, Forwarder: "0xa3d1ad4ac559a6575a114998affb2fb2ec97a7d9"},

	// Base
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA_BASE_1.Selector, Forwarder: "0x82300bd7c3958625581cc2f77bc6464dcecdf3e5"},
	{Selector: chainselectors.ETHEREUM_MAINNET_BASE_1.Selector, Forwarder: "0x5e342a8438b4f5d39e72875fcee6f76b39cce548"},

	// Avalanche
	{Selector: chainselectors.AVALANCHE_TESTNET_FUJI.Selector, Forwarder: "0x2e7371a5d032489e4f60216d8d898a4c10805963"},
	{Selector: chainselectors.AVALANCHE_MAINNET.Selector, Forwarder: "0xdc21e279934ff6721cadfdd112dafb3261f09a2c"},

	// Polygon
	{Selector: chainselectors.POLYGON_TESTNET_AMOY.Selector, Forwarder: "0x3675a5eb2286a3f87e8278fc66edf458a2e3bb74"},
	{Selector: chainselectors.POLYGON_MAINNET.Selector, Forwarder: "0xf458d621885e29a5003ea9bbba5280d54e19b1ce"},

	// BNB Chain
	{Selector: chainselectors.BINANCE_SMART_CHAIN_TESTNET.Selector, Forwarder: "0xa238e42cb8782808dbb2f37e19859244ec4779b0"},
	{Selector: chainselectors.BINANCE_SMART_CHAIN_MAINNET.Selector, Forwarder: "0x6f3239bbb26e98961e1115aba83f8a282e5508c8"},

	// Arbitrum
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA_ARBITRUM_1.Selector, Forwarder: "0xd41263567ddfead91504199b8c6c87371e83ca5d"},
	{Selector: chainselectors.ETHEREUM_MAINNET_ARBITRUM_1.Selector, Forwarder: "0xd770499057619c9a76205fd4168161cf94abc532"},

	// Optimism
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA_OPTIMISM_1.Selector, Forwarder: "0xa2888380dff3704a8ab6d1cd1a8f69c15fea5ee3"},
	{Selector: chainselectors.ETHEREUM_MAINNET_OPTIMISM_1.Selector, Forwarder: "0x9119a1501550ed94a3f2794038ed9258337afa18"},

	// Andesite (private testnet)
	{Selector: chainselectors.PRIVATE_TESTNET_ANDESITE.Selector, Forwarder: "0xcF4629d8DC7a5fa17F4D77233F5b953225669821"},

	// ZkSync
	{Selector: chainselectors.ETHEREUM_MAINNET_ZKSYNC_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA_ZKSYNC_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Jovay
	{Selector: chainselectors.JOVAY_TESTNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.JOVAY_MAINNET.Selector, Forwarder: "0x2B3068C4B288A2CD1f8B3613b8f33ef7cEecadC4"},

	// Pharos
	{Selector: chainselectors.PHAROS_ATLANTIC_TESTNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.PHAROS_MAINNET.Selector, Forwarder: "0x2B3068C4B288A2CD1f8B3613b8f33ef7cEecadC4"},

	// Worldchain
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA_WORLDCHAIN_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.ETHEREUM_MAINNET_WORLDCHAIN_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Plasma
	{Selector: chainselectors.PLASMA_TESTNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.PLASMA_MAINNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Linea
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA_LINEA_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.ETHEREUM_MAINNET_LINEA_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Ink
	{Selector: chainselectors.INK_TESTNET_SEPOLIA.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.ETHEREUM_MAINNET_INK_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Hyperliquid
	{Selector: chainselectors.HYPERLIQUID_TESTNET.Selector, Forwarder: "0xB27fA1c28288c50542527F64BCda22C9FbAc24CB"},
	{Selector: chainselectors.HYPERLIQUID_MAINNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Apechain
	{Selector: chainselectors.APECHAIN_TESTNET_CURTIS.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Arc
	{Selector: chainselectors.ARC_TESTNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Xlayer
	{Selector: chainselectors.XLAYER_TESTNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.ETHEREUM_MAINNET_XLAYER_1.Selector, Forwarder: "0x2B3068C4B288A2CD1f8B3613b8f33ef7cEecadC4"},

	// MegaETH
	{Selector: chainselectors.MEGAETH_TESTNET_2.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.MEGAETH_MAINNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Celo
	// {Selector: chainselectors.CELO_SEPOLIA.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.CELO_MAINNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Gnosis
	{Selector: chainselectors.GNOSIS_CHAIN_TESTNET_CHIADO.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.GNOSIS_CHAIN_MAINNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Cronos
	{Selector: chainselectors.CRONOS_TESTNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Mantle
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA_MANTLE_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.ETHEREUM_MAINNET_MANTLE_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// TAC
	{Selector: chainselectors.TAC_TESTNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Unichain
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA_UNICHAIN_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Scroll
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA_SCROLL_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.ETHEREUM_MAINNET_SCROLL_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Sonic
	{Selector: chainselectors.SONIC_TESTNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
	{Selector: chainselectors.SONIC_MAINNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// DTCC
	{Selector: chainselectors.DTCC_TESTNET_ANDESITE.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
}

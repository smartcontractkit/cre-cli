package simulate

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

const WorkflowExecutionTimeout = 5 * time.Minute

type ChainSelector = uint64

type ChainConfig struct {
	Selector  ChainSelector
	Forwarder string
}

// SupportedEVM is the canonical list you can range over.
var SupportedEVM = []ChainConfig{
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

	// Pharos
	// Integration not ready yet
	// {Selector: chainselectors.PHAROS_ATLANTIC_TESTNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Worldchain
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA_WORLDCHAIN_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Plasma
	{Selector: chainselectors.PLASMA_TESTNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Linea
	{Selector: chainselectors.ETHEREUM_TESTNET_SEPOLIA_LINEA_1.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Ink
	{Selector: chainselectors.INK_TESTNET_SEPOLIA.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Hyperliquid
	{Selector: chainselectors.HYPERLIQUID_TESTNET.Selector, Forwarder: "0xB27fA1c28288c50542527F64BCda22C9FbAc24CB"},

	// Apechain
	{Selector: chainselectors.APECHAIN_TESTNET_CURTIS.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},

	// Arc
	{Selector: chainselectors.ARC_TESTNET.Selector, Forwarder: "0x6E9EE680ef59ef64Aa8C7371279c27E496b5eDc1"},
}

// parse "ChainSelector:<digits>" from trigger id, e.g. "evm:ChainSelector:5009297550715157269@1.0.0 LogTrigger"
var chainSelectorRe = regexp.MustCompile(`(?i)chainselector:(\d+)`)

func parseChainSelectorFromTriggerID(id string) (uint64, bool) {
	m := chainSelectorRe.FindStringSubmatch(id)
	if len(m) < 2 {
		return 0, false
	}

	v, err := strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return 0, false
	}

	return v, true
}

// runRPCHealthCheck runs connectivity check against every configured client.
// experimentalForwarders keys identify experimental chains (not in chain-selectors).
func runRPCHealthCheck(clients map[uint64]*ethclient.Client, experimentalForwarders map[uint64]common.Address) error {
	if len(clients) == 0 {
		return fmt.Errorf("check your settings: no RPC URLs found for supported or experimental chains")
	}

	var errs []error
	for selector, c := range clients {
		if c == nil {
			// shouldnt happen
			errs = append(errs, fmt.Errorf("[%d] nil client", selector))
			continue
		}

		// Determine chain label for error messages
		var chainLabel string
		if _, isExperimental := experimentalForwarders[selector]; isExperimental {
			chainLabel = fmt.Sprintf("experimental chain %d", selector)
		} else {
			name, err := settings.GetChainNameByChainSelector(selector)
			if err != nil {
				// If we can't get the name, use the selector as the label
				chainLabel = fmt.Sprintf("chain %d", selector)
			} else {
				chainLabel = name
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		chainID, err := c.ChainID(ctx)
		cancel() // don't defer in a loop

		if err != nil {
			errs = append(errs, fmt.Errorf("[%s] failed RPC health check: %w", chainLabel, err))
			continue
		}
		if chainID == nil || chainID.Sign() <= 0 {
			errs = append(errs, fmt.Errorf("[%s] invalid RPC response: empty or zero chain ID", chainLabel))
			continue
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("RPC health check failed:\n%w", errors.Join(errs...))
	}
	return nil
}

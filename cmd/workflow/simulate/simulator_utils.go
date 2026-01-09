package simulate

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

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
	{Selector: chainselectors.PRIVATE_TESTNET_ANDESITE.Selector, Forwarder: "<PUT MOCK FORWARDER ADDRESS>"},
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
func runRPCHealthCheck(clients map[uint64]*ethclient.Client) error {
	if len(clients) == 0 {
		return fmt.Errorf("check your settings: no RPC URLs found for supported chains")
	}

	var errs []error
	for selector, c := range clients {
		if c == nil {
			// shouldnt happen
			errs = append(errs, fmt.Errorf("[%d] nil client", selector))
			continue
		}

		chainName, err := settings.GetChainNameByChainSelector(selector)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		chainID, err := c.ChainID(ctx)
		cancel() // don't defer in a loop

		if err != nil {
			errs = append(errs, fmt.Errorf("[%s] failed RPC health check: %w", chainName, err))
			continue
		}
		if chainID == nil || chainID.Sign() <= 0 {
			errs = append(errs, fmt.Errorf("[%s] invalid RPC response: empty or zero chain ID", chainName))
			continue
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("RPC health check failed:\n%w", errors.Join(errs...))
	}
	return nil
}

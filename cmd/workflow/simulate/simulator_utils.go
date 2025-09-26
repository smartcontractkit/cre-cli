package simulate

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

const TIMEOUT = 30 * time.Second
const (
	SEPOLIA_MOCK_KEYSTONE_FORWARDER_ADDRESS = "0x15fC6ae953E024d975e77382eEeC56A9101f9F88"
	MAINNET_MOCK_KEYSTONE_FORWARDER_ADDRESS = "0xa3d1ad4ac559a6575a114998affb2fb2ec97a7d9"
	SEPOLIA_CHAIN_SELECTOR                  = 16015286601757825753
	MAINNET_CHAIN_SELECTOR                  = 5009297550715157269
	SEPOLIA_CHAIN_NAME                      = "ethereum-testnet-sepolia"
	MAINNET_CHAIN_NAME                      = "ethereum-mainnet"
)

// ---- SUPPORTED CHAINS ----
var supportedEVM = []struct {
	Selector  uint64
	ChainName string
	Forwarder string
}{
	{Selector: SEPOLIA_CHAIN_SELECTOR, ChainName: SEPOLIA_CHAIN_NAME, Forwarder: SEPOLIA_MOCK_KEYSTONE_FORWARDER_ADDRESS},
	{Selector: MAINNET_CHAIN_SELECTOR, ChainName: MAINNET_CHAIN_NAME, Forwarder: MAINNET_MOCK_KEYSTONE_FORWARDER_ADDRESS},
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

package evm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// RunRPCHealthCheck validates RPC connectivity for all configured EVM clients.
// The experimentalSelectors set identifies which selectors are experimental chains.
func RunRPCHealthCheck(clients map[uint64]chain.ChainClient, experimentalSelectors map[uint64]bool) error {
	ethClients := make(map[uint64]*ethclient.Client)
	for sel, c := range clients {
		ec, ok := c.(*ethclient.Client)
		if !ok {
			return fmt.Errorf("[%d] invalid client type for EVM chain type", sel)
		}
		ethClients[sel] = ec
	}

	return checkRPCConnectivity(ethClients, experimentalSelectors)
}

// checkRPCConnectivity runs connectivity check against every configured client.
// experimentalSelectors set identifies experimental chains (not in chain-selectors).
func checkRPCConnectivity(clients map[uint64]*ethclient.Client, experimentalSelectors map[uint64]bool) error {
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
		if experimentalSelectors[selector] {
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

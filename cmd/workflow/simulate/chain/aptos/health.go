package aptos

import (
	"errors"
	"fmt"

	aptosfakes "github.com/smartcontractkit/chainlink-aptos/fakes"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// RunRPCHealthCheck probes GetChainId() on every configured Aptos client.
// experimentalSelectors identifies chains sourced from experimental-chains config.
func RunRPCHealthCheck(clients map[uint64]chain.ChainClient, experimentalSelectors map[uint64]bool) error {
	if len(clients) == 0 {
		return fmt.Errorf("no Aptos RPC URLs found for supported or experimental chains")
	}
	var errs []error
	for sel, c := range clients {
		if c == nil {
			errs = append(errs, fmt.Errorf("[%d] nil client", sel))
			continue
		}
		ac, ok := c.(aptosfakes.AptosClient)
		if !ok {
			errs = append(errs, fmt.Errorf("[%d] invalid client type for Aptos chain type", sel))
			continue
		}
		var label string
		switch {
		case experimentalSelectors[sel]:
			label = fmt.Sprintf("experimental chain %d", sel)
		default:
			if name, err := settings.GetChainNameByChainSelector(sel); err == nil {
				label = name
			} else {
				label = fmt.Sprintf("chain %d", sel)
			}
		}
		chainID, err := ac.GetChainId()
		if err != nil {
			errs = append(errs, fmt.Errorf("[%s] failed RPC health check: %w", label, err))
			continue
		}
		if chainID == 0 {
			errs = append(errs, fmt.Errorf("[%s] invalid RPC response: zero chain ID", label))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

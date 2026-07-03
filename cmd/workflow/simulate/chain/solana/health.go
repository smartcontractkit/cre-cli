package solana

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go/rpc"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

const healthCheckTimeout = 5 * time.Second

// RunRPCHealthCheck probes GetHealth() on every configured Solana client.
// experimentalSelectors identifies chains sourced from experimental-chains config.
func RunRPCHealthCheck(clients map[uint64]chain.ChainClient, experimentalSelectors map[uint64]bool) error {
	if len(clients) == 0 {
		return fmt.Errorf("check your settings: no Solana RPC URLs found for supported or experimental chains")
	}
	var errs []error
	for sel, c := range clients {
		if c == nil {
			errs = append(errs, fmt.Errorf("[%d] nil client", sel))
			continue
		}
		sc, ok := c.(*rpc.Client)
		if !ok {
			errs = append(errs, fmt.Errorf("[%d] invalid client type for Solana chain type", sel))
			continue
		}
		label := selectorLabel(sel, experimentalSelectors)

		ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
		// GetHealth returns "ok" on healthy nodes and an error otherwise.
		if _, err := sc.GetHealth(ctx); err != nil {
			errs = append(errs, fmt.Errorf("[%s] failed RPC health check: %w", label, err))
		}
		cancel()
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func selectorLabel(sel uint64, experimentalSelectors map[uint64]bool) string {
	if experimentalSelectors[sel] {
		return fmt.Sprintf("experimental chain %d", sel)
	}
	if name, err := settings.GetChainNameByChainSelector(sel); err == nil {
		return name
	}
	return fmt.Sprintf("chain %d", sel)
}

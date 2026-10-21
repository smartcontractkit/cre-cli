package stellar

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/stellar/go-stellar-sdk/clients/rpcclient"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

const healthCheckTimeout = 5 * time.Second

// RunRPCHealthCheck probes getHealth on every configured Stellar RPC client.
// experimentalSelectors identifies chains sourced from experimental-chains config.
func RunRPCHealthCheck(clients map[uint64]chain.ChainClient, experimentalSelectors map[uint64]bool) error {
	if len(clients) == 0 {
		return fmt.Errorf("check your settings: no Stellar RPC URLs found for supported or experimental chains")
	}
	var errs []error
	for sel, c := range clients {
		if c == nil {
			errs = append(errs, fmt.Errorf("[%d] nil client", sel))
			continue
		}
		sc, ok := c.(*rpcclient.Client)
		if !ok {
			errs = append(errs, fmt.Errorf("[%d] invalid client type for Stellar chain type", sel))
			continue
		}
		label := selectorLabel(sel, experimentalSelectors)

		ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
		res, err := sc.GetHealth(ctx)
		cancel()
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("[%s] failed RPC health check: %w", label, err))
		case res.Status != "healthy":
			errs = append(errs, fmt.Errorf("[%s] RPC reports status %q", label, res.Status))
		}
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

package aptos

import (
	"context"
	"fmt"

	"github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/crypto"

	aptosserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/aptos/server"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"

	aptosfakes "github.com/smartcontractkit/chainlink-aptos/fakes"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// AptosChainCapabilities holds the per-selector FakeAptosChain instances
// created for simulation.
type AptosChainCapabilities struct {
	AptosChains map[uint64]*aptosfakes.FakeAptosChain
}

// NewAptosChainCapabilities builds FakeAptosChain instances for every
// (selector -> client) pair, optionally wraps them with LimitedAptosChain,
// and registers each with the capability registry.
func NewAptosChainCapabilities(
	ctx context.Context,
	lggr logger.Logger,
	registry *capabilities.Registry,
	clients map[uint64]aptosfakes.AptosClient,
	forwarders map[uint64]string,
	privateKey *crypto.Ed25519PrivateKey,
	dryRunChainWrite bool,
	limits chain.Limits,
) (*AptosChainCapabilities, error) {
	chains := make(map[uint64]*aptosfakes.FakeAptosChain)
	for sel, client := range clients {
		fwdStr, ok := forwarders[sel]
		if !ok {
			lggr.Infow("Forwarder not found for chain", "selector", sel)
			continue
		}
		var fwd aptos.AccountAddress
		if err := fwd.ParseStringRelaxed(fwdStr); err != nil {
			return nil, fmt.Errorf("parse forwarder for selector %d: %w", sel, err)
		}
		fc, err := aptosfakes.NewFakeAptosChain(lggr, client, privateKey, fwd, sel, dryRunChainWrite)
		if err != nil {
			return nil, fmt.Errorf("new FakeAptosChain for selector %d: %w", sel, err)
		}
		capability := NewLimitedAptosChain(fc, limits)
		server := aptosserver.NewClientServer(capability)
		if err := registry.Add(ctx, server); err != nil {
			return nil, fmt.Errorf("register aptos capability for selector %d: %w", sel, err)
		}
		chains[sel] = fc
	}
	return &AptosChainCapabilities{AptosChains: chains}, nil
}

func (c *AptosChainCapabilities) Start(ctx context.Context) error {
	for _, fc := range c.AptosChains {
		if err := fc.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *AptosChainCapabilities) Close() error {
	for _, fc := range c.AptosChains {
		if err := fc.Close(); err != nil {
			return err
		}
	}
	return nil
}

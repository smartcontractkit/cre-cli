package solana

import (
	"context"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	solanaserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/solana/server"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	solanafakes "github.com/smartcontractkit/chainlink-solana/contracts/capabilities/fakes"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

// SolanaChainCapabilities holds the per-selector Solana chains created for
// simulation. Each is a ManualSolanaChain wrapping the fake chain so the
// simulator can fire log triggers manually.
type SolanaChainCapabilities struct {
	SolanaChains map[uint64]*ManualSolanaChain
}

// NewSolanaChainCapabilities builds FakeSolanaChain instances for every
// (selector -> client) pair, wraps them with LimitedSolanaChain, and registers
// each with the capability registry.
func NewSolanaChainCapabilities(
	ctx context.Context,
	lggr logger.Logger,
	registry *capabilities.Registry,
	clients map[uint64]*rpc.Client,
	forwarderProgramIDs map[uint64]solana.PublicKey,
	forwarderStateAccounts map[uint64]solana.PublicKey,
	transmitter solana.PrivateKey,
	dryRunChainWrite bool,
	limits chain.Limits,
) (*SolanaChainCapabilities, error) {
	chains := make(map[uint64]*ManualSolanaChain)
	for sel, client := range clients {
		programID, ok := forwarderProgramIDs[sel]
		if !ok {
			lggr.Infow("Forwarder program ID not found for chain", "selector", sel)
			continue
		}
		stateAccount, ok := forwarderStateAccounts[sel]
		if !ok {
			lggr.Infow("Forwarder state account not found for chain", "selector", sel)
			continue
		}
		fc, err := solanafakes.NewFakeSolanaChain(lggr, client, transmitter, programID, stateAccount, sel, dryRunChainWrite)
		if err != nil {
			return nil, fmt.Errorf("new FakeSolanaChain for selector %d: %w", sel, err)
		}
		capability := NewLimitedSolanaChain(fc, limits)
		manual := NewManualSolanaChain(capability)
		server := solanaserver.NewClientServer(manual)
		if err := registry.Add(ctx, server); err != nil {
			return nil, fmt.Errorf("register solana capability for selector %d: %w", sel, err)
		}
		chains[sel] = manual
	}
	return &SolanaChainCapabilities{SolanaChains: chains}, nil
}

func (c *SolanaChainCapabilities) Start(ctx context.Context) error {
	for _, fc := range c.SolanaChains {
		if err := fc.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *SolanaChainCapabilities) Close() error {
	for _, fc := range c.SolanaChains {
		if err := fc.Close(); err != nil {
			return err
		}
	}
	return nil
}

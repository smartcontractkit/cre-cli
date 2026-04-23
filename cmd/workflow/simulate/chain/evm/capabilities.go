package evm

import (
	"context"
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	evmserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm/server"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/fakes"
)

// EVMChainCapabilities holds the EVM chain capability servers created for simulation.
type EVMChainCapabilities struct {
	EVMChains map[uint64]*fakes.FakeEVMChain
}

// NewEVMChainCapabilities creates EVM chain capability servers and registers them
// with the capability registry. Cron and HTTP triggers are not created here — they
// are chain-agnostic and managed by the simulate command directly.
func NewEVMChainCapabilities(
	ctx context.Context,
	lggr logger.Logger,
	registry *capabilities.Registry,
	clients map[uint64]*ethclient.Client,
	forwarders map[uint64]string,
	privateKey *ecdsa.PrivateKey,
	dryRunChainWrite bool,
	limits EVMChainLimits,
) (*EVMChainCapabilities, error) {
	evmChains := make(map[uint64]*fakes.FakeEVMChain)
	for sel, client := range clients {
		fwdStr, ok := forwarders[sel]
		if !ok {
			lggr.Infow("Forwarder not found for chain", "selector", sel)
			continue
		}

		evm := fakes.NewFakeEvmChain(
			lggr,
			client,
			privateKey,
			common.HexToAddress(fwdStr),
			sel,
			dryRunChainWrite,
		)

		// Wrap with limits enforcement if limits are provided
		var evmCap evmserver.ClientCapability = evm
		if limits != nil {
			evmCap = NewLimitedEVMChain(evm, limits)
		}

		evmServer := evmserver.NewClientServer(evmCap)
		if err := registry.Add(ctx, evmServer); err != nil {
			return nil, err
		}

		evmChains[sel] = evm
	}

	return &EVMChainCapabilities{
		EVMChains: evmChains,
	}, nil
}

// Start starts all configured EVM chains.
func (c *EVMChainCapabilities) Start(ctx context.Context) error {
	for _, evm := range c.EVMChains {
		if err := evm.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Close closes all EVM chains.
func (c *EVMChainCapabilities) Close() error {
	for _, evm := range c.EVMChains {
		if err := evm.Close(); err != nil {
			return err
		}
	}
	return nil
}

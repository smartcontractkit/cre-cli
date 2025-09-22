package simulate

import (
	"context"
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	httpserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/http/server"
	evmserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm/server"
	consensusserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/consensus/server"
	crontrigger "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/cron/server"
	httptrigger "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/http/server"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/fakes"
	"github.com/smartcontractkit/chainlink/v2/core/services/keystore/chaintype"
	"github.com/smartcontractkit/chainlink/v2/core/services/keystore/keys/ocr2key"
)

type ManualTriggerCapabilitiesConfig struct {
	Clients    map[uint64]*ethclient.Client
	Forwarders map[uint64]common.Address
	PrivateKey *ecdsa.PrivateKey
}

type ManualTriggers struct {
	ManualCronTrigger *fakes.ManualCronTriggerService
	ManualHTTPTrigger *fakes.ManualHTTPTriggerService
	ManualEVMChains   map[uint64]*fakes.FakeEVMChain
}

func NewManualTriggerCapabilities(
	ctx context.Context,
	lggr logger.Logger,
	registry *capabilities.Registry,
	cfg ManualTriggerCapabilitiesConfig,
	dryRunChainWrite bool,
) (*ManualTriggers, error) {
	// Cron
	manualCronTrigger := fakes.NewManualCronTriggerService(lggr)
	manualCronTriggerServer := crontrigger.NewCronServer(manualCronTrigger)
	if err := registry.Add(ctx, manualCronTriggerServer); err != nil {
		return nil, err
	}

	// HTTP
	manualHTTPTrigger := fakes.NewManualHTTPTriggerService(lggr)
	manualHTTPTriggerServer := httptrigger.NewHTTPServer(manualHTTPTrigger)
	if err := registry.Add(ctx, manualHTTPTriggerServer); err != nil {
		return nil, err
	}

	// EVM
	evmChains := make(map[uint64]*fakes.FakeEVMChain)
	for sel, client := range cfg.Clients {
		fwd, ok := cfg.Forwarders[sel]
		if !ok {
			lggr.Infow("Forwarder not found for chain", "selector", sel)
			continue
		}

		evm := fakes.NewFakeEvmChain(
			lggr,
			client,
			cfg.PrivateKey,
			fwd,
			sel,
			dryRunChainWrite,
		)

		evmServer := evmserver.NewClientServer(evm)
		if err := registry.Add(ctx, evmServer); err != nil {
			return nil, err
		}

		evmChains[sel] = evm
	}

	return &ManualTriggers{
		ManualCronTrigger: manualCronTrigger,
		ManualHTTPTrigger: manualHTTPTrigger,
		ManualEVMChains:   evmChains,
	}, nil
}

func (m *ManualTriggers) Start(ctx context.Context) error {
	err := m.ManualCronTrigger.Start(ctx)
	if err != nil {
		return err
	}

	err = m.ManualHTTPTrigger.Start(ctx)
	if err != nil {
		return err
	}

	// Start all configured EVM chains
	for _, evm := range m.ManualEVMChains {
		if err := evm.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (m *ManualTriggers) Close() error {
	err := m.ManualCronTrigger.Close()
	if err != nil {
		return err
	}

	err = m.ManualHTTPTrigger.Close()
	if err != nil {
		return err
	}

	// Close all EVM chains
	for _, evm := range m.ManualEVMChains {
		if err := evm.Close(); err != nil {
			return err
		}
	}
	return nil
}

// NewFakeCapabilities builds faked capabilities, then registers them with the capability registry.
func NewFakeActionCapabilities(ctx context.Context, lggr logger.Logger, registry *capabilities.Registry) ([]services.Service, error) {
	caps := make([]services.Service, 0)

	// Consensus
	// generate deterministic signers - need to be configured on the Forwarder contract
	nSigners := 4
	signers := []ocr2key.KeyBundle{}
	for i := 0; i < nSigners; i++ {
		signer := ocr2key.MustNewInsecure(fakes.SeedForKeys(), chaintype.EVM)
		lggr.Infow("Generated new consensus signer", "addrss", common.BytesToAddress(signer.PublicKey()))
		signers = append(signers, signer)
	}
	fakeConsensusNoDAG := fakes.NewFakeConsensusNoDAG(signers, lggr)
	fakeConsensusServer := consensusserver.NewConsensusServer(fakeConsensusNoDAG)
	if err := registry.Add(ctx, fakeConsensusServer); err != nil {
		return nil, err
	}
	caps = append(caps, fakeConsensusServer)

	// HTTP Action
	httpAction := fakes.NewDirectHTTPAction(lggr)
	httpActionServer := httpserver.NewClientServer(httpAction)
	if err := registry.Add(ctx, httpActionServer); err != nil {
		return nil, err
	}
	caps = append(caps, httpActionServer)

	return caps, nil
}

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

const (
	MOCK_KEYSTONE_FORWARDER_ADDRESS = "0x15fC6ae953E024d975e77382eEeC56A9101f9F88"
	SEPOLIA_CHAIN_SELECTOR          = 16015286601757825753
)

type ManualTriggerCapabilitiesConfig struct {
	Client     *ethclient.Client
	PrivateKey *ecdsa.PrivateKey
}

type ManualTriggers struct {
	ManualCronTrigger     *fakes.ManualCronTriggerService
	ManualHTTPTrigger     *fakes.ManualHTTPTriggerService
	ManualEVMChainTrigger *fakes.FakeEVMChain
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
	evm := fakes.NewFakeEvmChain(
		lggr,
		cfg.Client,
		cfg.PrivateKey,
		common.HexToAddress(MOCK_KEYSTONE_FORWARDER_ADDRESS),
		SEPOLIA_CHAIN_SELECTOR,
		dryRunChainWrite,
	)
	evmServer := evmserver.NewClientServer(evm)
	if err := registry.Add(ctx, evmServer); err != nil {
		return nil, err
	}

	return &ManualTriggers{
		ManualCronTrigger:     manualCronTrigger,
		ManualHTTPTrigger:     manualHTTPTrigger,
		ManualEVMChainTrigger: evm,
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

	return nil
}

// NewFakeCapabilities builds faked capabilities, then registers them with the capability registry.
func NewFakeActionCapabilities(ctx context.Context, lggr logger.Logger, registry *capabilities.Registry) ([]services.Service, error) {
	caps := make([]services.Service, 0)

	// Consensus
	// generate deterministic signers - need to be configured on the Forwarder contract
	nSigners := 4
	signers := []ocr2key.KeyBundle{}
	for range nSigners {
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

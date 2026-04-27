package simulate

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/fakes/gateway"

	corekeys "github.com/smartcontractkit/chainlink-common/keystore/corekeys"
	"github.com/smartcontractkit/chainlink-common/keystore/corekeys/ocr2key"
	confhttpserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/confidentialhttp/server"
	httpserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/http/server"
	consensusserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/consensus/server"
	crontrigger "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/cron/server"
	httptrigger "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/http/server"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/fakes"
)

const (
	defaultLocalGatewayPort = 9090
)

// ManualTriggers holds chain-agnostic trigger services used in simulation.
type ManualTriggers struct {
	ManualCronTrigger *fakes.ManualCronTriggerService
	ManualHTTPTrigger *fakes.ManualHTTPTriggerService
}

// NewManualTriggerCapabilities creates and registers cron and HTTP trigger capabilities.
// These are chain-agnostic and shared across all chain types.
func NewManualTriggerCapabilities(ctx context.Context, lggr logger.Logger, registry *capabilities.Registry) (*ManualTriggers, error) {
	manualCronTrigger, err := fakes.NewManualCronTriggerService(lggr)
	if err != nil {
		return nil, err
	}
	manualCronTriggerServer := crontrigger.NewCronServer(manualCronTrigger)
	if err := registry.Add(ctx, manualCronTriggerServer); err != nil {
		return nil, err
	}

	manualHTTPTrigger := fakes.NewManualHTTPTriggerService(lggr, gateway.Config{
		Port: defaultLocalGatewayPort,
	})
	manualHTTPTriggerServer := httptrigger.NewHTTPServer(manualHTTPTrigger)
	if err := registry.Add(ctx, manualHTTPTriggerServer); err != nil {
		return nil, err
	}

	return &ManualTriggers{
		ManualCronTrigger: manualCronTrigger,
		ManualHTTPTrigger: manualHTTPTrigger,
	}, nil
}

// Start starts cron and HTTP trigger services.
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

// Close closes cron and HTTP trigger services.
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

// NewFakeActionCapabilities builds faked capabilities, then registers them with the capability registry.
func NewFakeActionCapabilities(ctx context.Context, lggr logger.Logger, registry *capabilities.Registry, secretsPath string, limits *SimulationLimits) ([]services.Service, error) {
	caps := make([]services.Service, 0)

	// Consensus
	// generate deterministic signers - need to be configured on the Forwarder contract
	nSigners := 4
	signers := []ocr2key.KeyBundle{}
	for i := 0; i < nSigners; i++ {
		signer := ocr2key.MustNewInsecure(fakes.SeedForKeys(), corekeys.EVM)
		lggr.Infow("Generated new consensus signer", "address", common.BytesToAddress(signer.PublicKey()))
		signers = append(signers, signer)
	}
	fakeConsensusNoDAG := fakes.NewFakeConsensusNoDAG(signers, lggr)
	var consensusCap consensusserver.ConsensusCapability = fakeConsensusNoDAG
	if limits != nil {
		consensusCap = NewLimitedConsensusNoDAG(fakeConsensusNoDAG, limits)
	}
	fakeConsensusServer := consensusserver.NewConsensusServer(consensusCap)
	if err := registry.Add(ctx, fakeConsensusServer); err != nil {
		return nil, err
	}
	caps = append(caps, fakeConsensusServer)

	// HTTP Action
	httpAction := fakes.NewDirectHTTPAction(lggr)
	var httpCap httpserver.ClientCapability = httpAction
	if limits != nil {
		httpCap = NewLimitedHTTPAction(httpAction, limits)
	}
	httpActionServer := httpserver.NewClientServer(httpCap)
	if err := registry.Add(ctx, httpActionServer); err != nil {
		return nil, err
	}
	caps = append(caps, httpActionServer)

	// Conf HTTP Action
	confHTTPAction := fakes.NewDirectConfidentialHTTPAction(lggr, secretsPath)
	var confHTTPCap confhttpserver.ClientCapability = confHTTPAction
	if limits != nil {
		confHTTPCap = NewLimitedConfidentialHTTPAction(confHTTPAction, limits)
	}
	confHTTPActionServer := confhttpserver.NewClientServer(confHTTPCap)
	if err := registry.Add(ctx, confHTTPActionServer); err != nil {
		return nil, err
	}
	caps = append(caps, confHTTPActionServer)

	return caps, nil
}

package simulate

import (
	"context"

	"github.com/ethereum/go-ethereum/common"

	cronservice "github.com/smartcontractkit/capabilities/scheduler/cron/service"
	corekeys "github.com/smartcontractkit/chainlink-common/keystore/corekeys"
	"github.com/smartcontractkit/chainlink-common/keystore/corekeys/ocr2key"
	capreg "github.com/smartcontractkit/chainlink-common/pkg/capabilities/registry"
	confhttpserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/confidentialhttp/server"
	httpserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/http/server"
	consensusserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/consensus/server"
	httptrigger "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/http/server"
	"github.com/smartcontractkit/chainlink-common/pkg/config"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/fakes"
)

// ManualTriggers holds chain-agnostic trigger services used in simulation.
type ManualTriggers struct {
	// Capabilities are the capabilities from the capabilities repo.
	Capabilities      *Capabilities
	ManualHTTPTrigger *ManualHTTPTriggerService
}

// NewManualTriggerCapabilities creates and registers cron and HTTP trigger capabilities.
// These are chain-agnostic and shared across all chain types.
func NewManualTriggerCapabilities(ctx context.Context, lggr logger.Logger, registry *capreg.Registry, caps *Capabilities, httpTriggerPort int, limits *SimulationLimits) (*ManualTriggers, error) {
	if err := caps.Add(ctx, registry, cronservice.New); err != nil {
		return nil, err
	}

	var httpTriggerRateLimit *config.Rate
	if limits != nil {
		rate := limits.HTTPTriggerRateLimit()
		httpTriggerRateLimit = &rate
	}
	manualHTTPTrigger := NewManualHTTPTriggerService(lggr, httpTriggerPort, httpTriggerRateLimit)
	manualHTTPTriggerServer := httptrigger.NewHTTPServer(manualHTTPTrigger)
	if err := registry.Add(ctx, manualHTTPTriggerServer); err != nil {
		return nil, err
	}

	return &ManualTriggers{
		Capabilities:      caps,
		ManualHTTPTrigger: manualHTTPTrigger,
	}, nil
}

// Start starts the HTTP trigger service. Capabilities from the capabilities repo are started when they are built.
func (m *ManualTriggers) Start(ctx context.Context) error {
	return m.ManualHTTPTrigger.Start(ctx)
}

// Close closes the HTTP trigger service. Capabilities from the capabilities repo are closed with their services.
func (m *ManualTriggers) Close() error {
	return m.ManualHTTPTrigger.Close()
}

// NewFakeActionCapabilities builds faked capabilities, then registers them with the capability registry.
func NewFakeActionCapabilities(ctx context.Context, lggr logger.Logger, registry *capreg.Registry, secretsPath string, limits *SimulationLimits) ([]services.Service, error) {
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
		confHTTPCap = NewLimitedConfidentialHTTPAction(confHTTPAction, limits, lggr)
	}
	confHTTPActionServer := confhttpserver.NewClientServer(confHTTPCap)
	if err := registry.Add(ctx, confHTTPActionServer); err != nil {
		return nil, err
	}
	caps = append(caps, confHTTPActionServer)

	return caps, nil
}

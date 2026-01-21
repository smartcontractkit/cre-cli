package simulate

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	confidentialhttp "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/confidentialhttp"
	confhttpserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/confidentialhttp/server"
	customhttp "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/http"
	httpserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/actions/http/server"
	evmserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm/server"
	consensusserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/consensus/server"
	crontrigger "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/cron/server"
	httptrigger "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/http/server"
	commoncap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	"github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
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

// debugHTTPWrapper wraps an HTTP capability to add debug logging for CRE-Ignore headers
type debugHTTPWrapper struct {
	httpserver.ClientCapability
	lggr logger.Logger
}

func (w *debugHTTPWrapper) SendRequest(ctx context.Context, metadata commoncap.RequestMetadata, input *customhttp.Request) (*commoncap.ResponseAndMetadata[*customhttp.Response], caperrors.Error) {
	// Check for CRE-Ignore header and log it
	if input != nil && input.GetHeaders() != nil {
		if creIgnoreValue, exists := input.GetHeaders()["CRE-Ignore"]; exists {
			fmt.Fprintf(os.Stderr, "[DEBUG] CRE-Ignore header value: %s\n", creIgnoreValue)
			w.lggr.Infow("[DEBUG] CRE-Ignore header value", "value", creIgnoreValue)
			// Remove the header before making the call
			delete(input.GetHeaders(), "CRE-Ignore")
		}
	}
	return w.ClientCapability.SendRequest(ctx, metadata, input)
}

// debugConfHTTPWrapper wraps a Confidential HTTP capability to add debug logging for CRE-Ignore headers
type debugConfHTTPWrapper struct {
	confhttpserver.ClientCapability
	lggr logger.Logger
}

func (w *debugConfHTTPWrapper) SendRequests(ctx context.Context, metadata commoncap.RequestMetadata, input *confidentialhttp.EnclaveActionInput) (*commoncap.ResponseAndMetadata[*confidentialhttp.HTTPEnclaveResponseData], caperrors.Error) {
	// Check for CRE-Ignore header in requests and log it
	if input != nil && input.GetInput() != nil {
		for i, req := range input.GetInput().GetRequests() {
			if req != nil && len(req.GetHeaders()) > 0 {
				for _, header := range req.GetHeaders() {
					// Headers are in "Key: Value" format
					if len(header) > 11 && header[:11] == "CRE-Ignore:" {
						value := header[11:]
						// Trim leading space if present
						if len(value) > 0 && value[0] == ' ' {
							value = value[1:]
						}
						w.lggr.Infow("CRE-Ignore header value", "value", value, "requestIndex", i)
						// Note: Header removal is handled by the underlying fake implementation
					}
				}
			}
		}
	}
	return w.ClientCapability.SendRequests(ctx, metadata, input)
}

// debugConsensusWrapper wraps a Consensus capability to add debug logging for map ID values
type debugConsensusWrapper struct {
	consensusserver.ConsensusCapability
	lggr logger.Logger
}

func (w *debugConsensusWrapper) Simple(ctx context.Context, metadata commoncap.RequestMetadata, input *sdk.SimpleConsensusInputs) (*commoncap.ResponseAndMetadata[*valuespb.Value], caperrors.Error) {
	// Check if consensus input is a map and log the ID value
	if input != nil {
		switch obs := input.Observation.(type) {
		case *sdk.SimpleConsensusInputs_Value:
			if obs.Value != nil {
				value, err := values.FromProto(obs.Value)
				if err == nil {
					unwrapped, err := value.Unwrap()
					if err == nil {
						if structMap, ok := unwrapped.(map[string]any); ok {
							if idVal, exists := structMap["id"]; exists {
								fmt.Fprintf(os.Stderr, "[DEBUG] Consensus on map - ID value: %v\n", idVal)
								w.lggr.Infow("[DEBUG] Consensus on map - ID value", "id", fmt.Sprintf("%v", idVal))
							}
						}
					}
				}
			}
		}
	}
	return w.ConsensusCapability.Simple(ctx, metadata, input)
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
		lggr.Infow("Generated new consensus signer", "address", common.BytesToAddress(signer.PublicKey()))
		signers = append(signers, signer)
	}
	fakeConsensusNoDAG := fakes.NewFakeConsensusNoDAG(signers, lggr)
	debugConsensus := &debugConsensusWrapper{
		ConsensusCapability: fakeConsensusNoDAG,
		lggr:                lggr,
	}
	fakeConsensusServer := consensusserver.NewConsensusServer(debugConsensus)
	if err := registry.Add(ctx, fakeConsensusServer); err != nil {
		return nil, err
	}
	caps = append(caps, fakeConsensusServer)

	// HTTP Action
	httpAction := fakes.NewDirectHTTPAction(lggr)
	debugHTTP := &debugHTTPWrapper{
		ClientCapability: httpAction,
		lggr:             lggr,
	}
	httpActionServer := httpserver.NewClientServer(debugHTTP)
	if err := registry.Add(ctx, httpActionServer); err != nil {
		return nil, err
	}
	caps = append(caps, httpActionServer)

	// Conf HTTP Action
	confHTTPAction := fakes.NewDirectConfidentialHTTPAction(lggr)
	debugConfHTTP := &debugConfHTTPWrapper{
		ClientCapability: confHTTPAction,
		lggr:             lggr,
	}
	confHTTPActionServer := confhttpserver.NewClientServer(debugConfHTTP)
	if err := registry.Add(ctx, confHTTPActionServer); err != nil {
		return nil, err
	}
	caps = append(caps, confHTTPActionServer)

	return caps, nil
}

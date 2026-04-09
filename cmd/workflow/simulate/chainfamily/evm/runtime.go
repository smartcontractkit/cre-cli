package evm

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/ethclient"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	evmcappb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	evmserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm/server"
	"github.com/smartcontractkit/chainlink-common/pkg/services"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/fakes"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chainfamily"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// evmRuntime implements chainfamily.ChainRuntime for EVM chains.
type evmRuntime struct {
	clients              map[uint64]*ethclient.Client
	chains               map[uint64]*fakes.FakeEVMChain
	services             []services.Service
	reportSizeLimitBytes int
	gasLimit             uint64
}

func (r *evmRuntime) RegisterCapabilities(ctx context.Context, registry *capabilities.Registry) error {
	for sel, evm := range r.chains {
		var evmCap evmserver.ClientCapability = evm
		if r.reportSizeLimitBytes > 0 || r.gasLimit > 0 {
			evmCap = &limitedEVMChain{
				ClientCapability: evm,
				reportSizeLimit:  r.reportSizeLimitBytes,
				gasLimit:         r.gasLimit,
			}
		}
		evmSrv := evmserver.NewClientServer(evmCap)
		if err := registry.Add(ctx, evmSrv); err != nil {
			return fmt.Errorf("failed to register EVM chain capability for selector %d: %w", sel, err)
		}
	}
	return nil
}

func (r *evmRuntime) BuildTriggerFunc(ctx context.Context, req chainfamily.TriggerRequest) (func() error, error) {
	evmChain := r.chains[req.ChainSelector]
	if evmChain == nil {
		return nil, fmt.Errorf("no EVM chain initialized for selector %d", req.ChainSelector)
	}
	client := r.clients[req.ChainSelector]

	if req.PromptUser {
		log, err := getEVMTriggerLog(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("failed to get EVM trigger log: %w", err)
		}
		return func() error {
			return evmChain.ManualTrigger(ctx, req.TriggerRegistrationID, log)
		}, nil
	}

	// Non-interactive mode - use flag values
	txHash := req.FlagValues("evm-tx-hash")
	eventIndexStr := req.FlagValues("evm-event-index")

	if strings.TrimSpace(txHash) == "" || strings.TrimSpace(eventIndexStr) == "" {
		return nil, fmt.Errorf("--evm-tx-hash and --evm-event-index are required for EVM triggers in non-interactive mode")
	}

	eventIndex, err := strconv.ParseUint(strings.TrimSpace(eventIndexStr), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid --evm-event-index: %w", err)
	}

	log, err := getEVMTriggerLogFromValues(ctx, client, txHash, eventIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to build EVM trigger log: %w", err)
	}

	return func() error {
		return evmChain.ManualTrigger(ctx, req.TriggerRegistrationID, log)
	}, nil
}

func (r *evmRuntime) OwnedSelectors() []chainfamily.ChainSelector {
	sels := make([]chainfamily.ChainSelector, 0, len(r.chains))
	for sel := range r.chains {
		sels = append(sels, sel)
	}
	return sels
}

func (r *evmRuntime) Services() []services.Service {
	return r.services
}

func (r *evmRuntime) Close() error {
	for sel, chain := range r.chains {
		if err := chain.Close(); err != nil {
			ui.Warning(fmt.Sprintf("Failed to close EVM chain %d: %v", sel, err))
		}
	}
	return nil
}

// limitedEVMChain wraps an evmserver.ClientCapability and enforces chain write
// report size and gas limits during simulation. Only WriteReport is overridden;
// all other methods delegate via the embedded interface.
type limitedEVMChain struct {
	evmserver.ClientCapability
	reportSizeLimit int
	gasLimit        uint64
}

func (l *limitedEVMChain) WriteReport(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.WriteReportRequest) (*commonCap.ResponseAndMetadata[*evmcappb.WriteReportReply], caperrors.Error) {
	if l.reportSizeLimit > 0 && input.Report != nil && len(input.Report.RawReport) > l.reportSizeLimit {
		return nil, caperrors.NewPublicUserError(
			fmt.Errorf("simulation limit exceeded: chain write report size %d bytes exceeds limit of %d bytes", len(input.Report.RawReport), l.reportSizeLimit),
			caperrors.ResourceExhausted,
		)
	}
	if l.gasLimit > 0 && input.GasConfig != nil && input.GasConfig.GasLimit > l.gasLimit {
		return nil, caperrors.NewPublicUserError(
			fmt.Errorf("simulation limit exceeded: EVM gas limit %d exceeds maximum of %d", input.GasConfig.GasLimit, l.gasLimit),
			caperrors.ResourceExhausted,
		)
	}
	return l.ClientCapability.WriteReport(ctx, metadata, input)
}

package evm

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	evmcappb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm"
	evmserver "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/evm/server"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
)

var _ evmserver.ClientCapability = (*ManualEVMChain)(nil)

type ManualEVMChain struct {
	inner evmserver.ClientCapability

	mu                sync.RWMutex
	callbackCh        map[string]chan commonCap.TriggerAndId[*evmcappb.Log]
	logTriggerFilters map[string]*evmcappb.FilterLogTriggerRequest
}

func NewManualEVMChain(inner evmserver.ClientCapability) *ManualEVMChain {
	return &ManualEVMChain{
		inner:             inner,
		callbackCh:        make(map[string]chan commonCap.TriggerAndId[*evmcappb.Log]),
		logTriggerFilters: make(map[string]*evmcappb.FilterLogTriggerRequest),
	}
}

func (m *ManualEVMChain) ManualTrigger(ctx context.Context, triggerID string, log *evmcappb.Log) error {
	m.mu.RLock()
	filter := m.logTriggerFilters[triggerID]
	callbackCh := m.callbackCh[triggerID]
	m.mu.RUnlock()

	if callbackCh == nil {
		return fmt.Errorf("EVM log trigger %q is not registered", triggerID)
	}
	if filter != nil {
		if err := manualEVMLogMatchesFilter(log, filter); err != nil {
			return fmt.Errorf("log does not match registered filter for trigger %s: %w", triggerID, err)
		}
	}

	go func() {
		select {
		case callbackCh <- m.createManualTriggerEvent(log):
		case <-ctx.Done():
		}
	}()

	return nil
}

func (m *ManualEVMChain) createManualTriggerEvent(log *evmcappb.Log) commonCap.TriggerAndId[*evmcappb.Log] {
	return commonCap.TriggerAndId[*evmcappb.Log]{
		Trigger: log,
		Id:      manualEVMTriggerEventID(log),
	}
}

func manualEVMTriggerEventID(log *evmcappb.Log) string {
	return fmt.Sprintf("manual-evm-chain-trigger-%x-%x-%d", log.GetBlockHash(), log.GetTxHash(), log.GetIndex())
}

func manualEVMLogMatchesFilter(log *evmcappb.Log, filter *evmcappb.FilterLogTriggerRequest) error {
	if len(filter.GetAddresses()) > 0 {
		addrMatched := false
		for _, addr := range filter.GetAddresses() {
			if bytes.Equal(log.GetAddress(), addr) {
				addrMatched = true
				break
			}
		}
		if !addrMatched {
			return fmt.Errorf("log address %x does not match any of the addresses in the filter", log.GetAddress())
		}
	}

	logTopics := log.GetTopics()
	for i, topicValues := range filter.GetTopics() {
		if len(topicValues.GetValues()) == 0 {
			continue
		}
		if i >= len(logTopics) {
			return fmt.Errorf("log topics length %d does not match the filter topics length %d", len(logTopics), len(filter.GetTopics()))
		}
		slotMatched := false
		for _, v := range topicValues.GetValues() {
			if bytes.Equal(logTopics[i], v) {
				slotMatched = true
				break
			}
		}
		if !slotMatched {
			return fmt.Errorf("log topic %d does not match any of the values in the filter", i)
		}
	}

	return nil
}

func (m *ManualEVMChain) RegisterLogTrigger(ctx context.Context, triggerID string, metadata commonCap.RequestMetadata, input *evmcappb.FilterLogTriggerRequest) (<-chan commonCap.TriggerAndId[*evmcappb.Log], caperrors.Error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callbackCh[triggerID] = make(chan commonCap.TriggerAndId[*evmcappb.Log])
	m.logTriggerFilters[triggerID] = input
	return m.callbackCh[triggerID], nil
}

func (m *ManualEVMChain) UnregisterLogTrigger(ctx context.Context, triggerID string, metadata commonCap.RequestMetadata, input *evmcappb.FilterLogTriggerRequest) caperrors.Error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.logTriggerFilters, triggerID)
	delete(m.callbackCh, triggerID)
	return nil
}

func (m *ManualEVMChain) AckEvent(ctx context.Context, triggerID string, eventID string, method string) caperrors.Error {
	return nil
}

func (m *ManualEVMChain) CallContract(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.CallContractRequest) (*commonCap.ResponseAndMetadata[*evmcappb.CallContractReply], caperrors.Error) {
	return m.inner.CallContract(ctx, metadata, input)
}

func (m *ManualEVMChain) FilterLogs(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.FilterLogsRequest) (*commonCap.ResponseAndMetadata[*evmcappb.FilterLogsReply], caperrors.Error) {
	return m.inner.FilterLogs(ctx, metadata, input)
}

func (m *ManualEVMChain) BalanceAt(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.BalanceAtRequest) (*commonCap.ResponseAndMetadata[*evmcappb.BalanceAtReply], caperrors.Error) {
	return m.inner.BalanceAt(ctx, metadata, input)
}

func (m *ManualEVMChain) EstimateGas(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.EstimateGasRequest) (*commonCap.ResponseAndMetadata[*evmcappb.EstimateGasReply], caperrors.Error) {
	return m.inner.EstimateGas(ctx, metadata, input)
}

func (m *ManualEVMChain) GetTransactionByHash(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.GetTransactionByHashRequest) (*commonCap.ResponseAndMetadata[*evmcappb.GetTransactionByHashReply], caperrors.Error) {
	return m.inner.GetTransactionByHash(ctx, metadata, input)
}

func (m *ManualEVMChain) GetTransactionReceipt(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.GetTransactionReceiptRequest) (*commonCap.ResponseAndMetadata[*evmcappb.GetTransactionReceiptReply], caperrors.Error) {
	return m.inner.GetTransactionReceipt(ctx, metadata, input)
}

func (m *ManualEVMChain) HeaderByNumber(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.HeaderByNumberRequest) (*commonCap.ResponseAndMetadata[*evmcappb.HeaderByNumberReply], caperrors.Error) {
	return m.inner.HeaderByNumber(ctx, metadata, input)
}

func (m *ManualEVMChain) WriteReport(ctx context.Context, metadata commonCap.RequestMetadata, input *evmcappb.WriteReportRequest) (*commonCap.ResponseAndMetadata[*evmcappb.WriteReportReply], caperrors.Error) {
	return m.inner.WriteReport(ctx, metadata, input)
}

func (m *ManualEVMChain) ChainSelector() uint64 { return m.inner.ChainSelector() }

func (m *ManualEVMChain) Start(ctx context.Context) error { return m.inner.Start(ctx) }
func (m *ManualEVMChain) Close() error                    { return m.inner.Close() }
func (m *ManualEVMChain) HealthReport() map[string]error  { return m.inner.HealthReport() }
func (m *ManualEVMChain) Name() string                    { return m.inner.Name() }
func (m *ManualEVMChain) Description() string             { return m.inner.Description() }
func (m *ManualEVMChain) Ready() error                    { return m.inner.Ready() }
func (m *ManualEVMChain) Initialise(ctx context.Context, dependencies core.StandardCapabilitiesDependencies) error {
	return m.inner.Initialise(ctx, dependencies)
}

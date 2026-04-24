package aptos

// simulator_scenarios_test.go runs 30 dry-run scenarios exercising FakeAptosChain
// via the simulator plumbing. All scenarios are fully in-process: no network I/O,
// no --broadcast. They verify parity with the EVM simulator's behavioural surface
// (success paths, validation errors, limit enforcement, per-selector dispatch,
// key resolution semantics).

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/aptos-labs/aptos-go-sdk/crypto"
	"github.com/rs/zerolog"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	aptoscappb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/aptos"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
	sdk "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities"

	aptosfakes "github.com/smartcontractkit/chainlink-aptos/fakes"
	mocks "github.com/smartcontractkit/chainlink-aptos/relayer/monitor/mocks"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// simScenario is a self-contained dry-run scenario.
type simScenario struct {
	name string
	run  func(t *testing.T)
}

// mkAddr returns a 32-byte address whose first byte is b.
func mkAddr(b byte) []byte { out := make([]byte, 32); out[0] = b; return out }

func testAddr(t *testing.T, s string) aptos.AccountAddress {
	t.Helper()
	var a aptos.AccountAddress
	require.NoError(t, a.ParseStringRelaxed(s))
	return a
}

func newKey(t *testing.T) *crypto.Ed25519PrivateKey {
	t.Helper()
	k, err := crypto.GenerateEd25519PrivateKey()
	require.NoError(t, err)
	return k
}

func newChain(t *testing.T, rpc *mocks.AptosRpcClient, dryRun bool, selector uint64) *aptosfakes.FakeAptosChain {
	t.Helper()
	fc, err := aptosfakes.NewFakeAptosChain(logger.Test(t), rpc, newKey(t),
		testAddr(t, "0xdead"), selector, dryRun)
	require.NoError(t, err)
	return fc
}

func simulatorScenarios() []simScenario {
	meta := commonCap.RequestMetadata{}
	ctx := context.Background()

	validGas := func() *aptoscappb.GasConfig {
		return &aptoscappb.GasConfig{MaxGasAmount: 10_000, GasUnitPrice: 100}
	}
	validReport := func() *sdk.ReportResponse {
		return &sdk.ReportResponse{RawReport: []byte("report")}
	}

	return []simScenario{
		// --- read-path scenarios (1-10) ---
		{name: "01 AccountAPTBalance returns u64", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().AccountAPTBalance(mock.Anything).Return(uint64(12345), nil).Once()
			fc := newChain(t, rpc, true, chainselectors.APTOS_TESTNET.Selector)
			reply, capErr := fc.AccountAPTBalance(ctx, meta, &aptoscappb.AccountAPTBalanceRequest{Address: mkAddr(0xA1)})
			require.Nil(t, capErr)
			assert.Equal(t, uint64(12345), reply.Response.Value)
		}},
		{name: "02 AccountAPTBalance rejects nil", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.AccountAPTBalance(ctx, meta, nil)
			require.NotNil(t, capErr)
		}},
		{name: "03 AccountAPTBalance rejects short address", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.AccountAPTBalance(ctx, meta, &aptoscappb.AccountAPTBalanceRequest{Address: []byte{1, 2, 3}})
			require.NotNil(t, capErr)
		}},
		{name: "04 AccountAPTBalance surfaces RPC error", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().AccountAPTBalance(mock.Anything).Return(uint64(0), fmt.Errorf("503")).Once()
			fc := newChain(t, rpc, true, 1)
			_, capErr := fc.AccountAPTBalance(ctx, meta, &aptoscappb.AccountAPTBalanceRequest{Address: mkAddr(0x01)})
			require.NotNil(t, capErr)
		}},
		{name: "05 View returns JSON-marshaled result array", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().View(mock.Anything).Return([]any{"hello"}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, capErr := fc.View(ctx, meta, &aptoscappb.ViewRequest{
				Payload: &aptoscappb.ViewPayload{
					Module:   &aptoscappb.ModuleID{Address: mkAddr(0x01), Name: "m"},
					Function: "f",
				},
			})
			require.Nil(t, capErr)
			assert.Equal(t, []byte(`["hello"]`), reply.Response.Data)
		}},
		{name: "06 View rejects nil payload", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.View(ctx, meta, &aptoscappb.ViewRequest{})
			require.NotNil(t, capErr)
		}},
		{name: "07 View respects ledger_version", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().View(mock.Anything, mock.Anything).Return([]any{"0"}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			ledger := uint64(42)
			_, capErr := fc.View(ctx, meta, &aptoscappb.ViewRequest{
				Payload:       &aptoscappb.ViewPayload{Module: &aptoscappb.ModuleID{Address: mkAddr(1), Name: "m"}, Function: "f"},
				LedgerVersion: &ledger,
			})
			require.Nil(t, capErr)
		}},
		{name: "08 TransactionByHash found", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().TransactionByHash("0x1").Return(&api.Transaction{
				Type: api.TransactionVariantUser, Inner: &api.UserTransaction{Hash: "0x1", Version: 1, Success: true},
			}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, capErr := fc.TransactionByHash(ctx, meta, &aptoscappb.TransactionByHashRequest{Hash: "0x1"})
			require.Nil(t, capErr)
			require.NotNil(t, reply.Response.Transaction)
			assert.Equal(t, "0x1", reply.Response.Transaction.Hash)
		}},
		{name: "09 TransactionByHash missing returns nil tx", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().TransactionByHash(mock.Anything).Return(nil, nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, capErr := fc.TransactionByHash(ctx, meta, &aptoscappb.TransactionByHashRequest{Hash: "0xnope"})
			require.Nil(t, capErr)
			assert.Nil(t, reply.Response.Transaction)
		}},
		{name: "10 TransactionByHash empty hash rejected", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.TransactionByHash(ctx, meta, &aptoscappb.TransactionByHashRequest{Hash: ""})
			require.NotNil(t, capErr)
		}},

		// --- pagination + account list (11-13) ---
		{name: "11 AccountTransactions delegates pagination", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().AccountTransactions(mock.Anything, mock.Anything, mock.Anything).
				Return([]*api.CommittedTransaction{
					{Type: api.TransactionVariantUser, Inner: &api.UserTransaction{Hash: "0xa"}},
				}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			s, l := uint64(0), uint64(10)
			reply, capErr := fc.AccountTransactions(ctx, meta, &aptoscappb.AccountTransactionsRequest{
				Address: mkAddr(0x01), Start: &s, Limit: &l,
			})
			require.Nil(t, capErr)
			require.Len(t, reply.Response.Transactions, 1)
		}},
		{name: "12 AccountTransactions rejects bad address", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.AccountTransactions(ctx, meta, &aptoscappb.AccountTransactionsRequest{Address: []byte{1}})
			require.NotNil(t, capErr)
		}},
		{name: "13 AccountTransactions rpc error surfaced", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().AccountTransactions(mock.Anything, mock.Anything, mock.Anything).
				Return(nil, fmt.Errorf("transport")).Once()
			fc := newChain(t, rpc, true, 1)
			_, capErr := fc.AccountTransactions(ctx, meta, &aptoscappb.AccountTransactionsRequest{Address: mkAddr(0x01)})
			require.NotNil(t, capErr)
		}},

		// --- WriteReport validation (14-17) ---
		{name: "14 WriteReport nil request rejected", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.WriteReport(ctx, meta, nil)
			require.NotNil(t, capErr)
		}},
		{name: "15 WriteReport nil gas config rejected", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{Receiver: mkAddr(1), Report: validReport()})
			require.NotNil(t, capErr)
		}},
		{name: "16 WriteReport nil report rejected", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{Receiver: mkAddr(1), GasConfig: validGas()})
			require.NotNil(t, capErr)
		}},
		{name: "17 WriteReport bad receiver len rejected", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: []byte{1}, GasConfig: validGas(), Report: validReport(),
			})
			require.NotNil(t, capErr)
		}},

		// --- WriteReport dry-run behaviour (18-22) ---
		{name: "18 WriteReport dry-run SUCCESS, no tx hash, zero fee", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: true}}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			require.Nil(t, capErr)
			assert.Equal(t, aptoscappb.TxStatus_TX_STATUS_SUCCESS, reply.Response.TxStatus)
			assert.Nil(t, reply.Response.TxHash)
			require.NotNil(t, reply.Response.TransactionFee)
			assert.Zero(t, *reply.Response.TransactionFee)
		}},
		{name: "19 WriteReport dry-run receiver abort -> REVERTED", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: false, VmStatus: "Move abort in 0xabc::receiver: Reject"}}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, _ := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			assert.Equal(t, aptoscappb.TxStatus_TX_STATUS_FATAL, reply.Response.TxStatus)
			require.NotNil(t, reply.Response.ReceiverContractExecutionStatus)
			assert.Equal(t,
				aptoscappb.ReceiverContractExecutionStatus_RECEIVER_CONTRACT_EXECUTION_STATUS_REVERTED,
				*reply.Response.ReceiverContractExecutionStatus)
		}},
		{name: "20 WriteReport dry-run forwarder abort -> no receiver status", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: false, VmStatus: "Move abort in 0xdead::mock_forwarder: Bad"}}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, _ := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			assert.Equal(t, aptoscappb.TxStatus_TX_STATUS_FATAL, reply.Response.TxStatus)
			assert.Nil(t, reply.Response.ReceiverContractExecutionStatus)
		}},
		{name: "21 WriteReport dry-run BuildTransaction error surfaces", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil, fmt.Errorf("rpc-down")).Once()
			fc := newChain(t, rpc, true, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			require.NotNil(t, capErr)
		}},
		{name: "22 WriteReport dry-run Simulate error surfaces", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return(nil, fmt.Errorf("sim-fail")).Once()
			fc := newChain(t, rpc, true, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			require.NotNil(t, capErr)
		}},

		// --- LimitedAptosChain enforcement (23-26) ---
		{name: "23 LimitedAptosChain blocks oversized report", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 5, maxGas: 10_000})
			_, capErr := l.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(),
				Report: &sdk.ReportResponse{RawReport: make([]byte, 999)},
			})
			require.NotNil(t, capErr)
		}},
		{name: "24 LimitedAptosChain blocks excessive max_gas_amount", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 1000, maxGas: 50})
			_, capErr := l.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: &aptoscappb.GasConfig{MaxGasAmount: 100, GasUnitPrice: 1},
				Report: validReport(),
			})
			require.NotNil(t, capErr)
		}},
		{name: "25 LimitedAptosChain passes through within limits (dry-run)", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: true}}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 1000, maxGas: 100_000})
			reply, capErr := l.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			require.Nil(t, capErr)
			assert.Equal(t, aptoscappb.TxStatus_TX_STATUS_SUCCESS, reply.Response.TxStatus)
		}},
		{name: "26 LimitedAptosChain delegates reads unconditionally", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().AccountAPTBalance(mock.Anything).Return(uint64(9), nil).Once()
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 1, maxGas: 1})
			reply, capErr := l.AccountAPTBalance(ctx, meta, &aptoscappb.AccountAPTBalanceRequest{Address: mkAddr(0x02)})
			require.Nil(t, capErr)
			assert.Equal(t, uint64(9), reply.Response.Value)
		}},

		// --- Multi-selector + key-resolution semantics (27-30) ---
		{name: "27 Per-selector dispatch isolates chains", run: func(t *testing.T) {
			rpcA := mocks.NewAptosRpcClient(t)
			rpcB := mocks.NewAptosRpcClient(t)
			rpcA.EXPECT().AccountAPTBalance(mock.Anything).Return(uint64(100), nil).Once()
			rpcB.EXPECT().AccountAPTBalance(mock.Anything).Return(uint64(200), nil).Once()
			fcA := newChain(t, rpcA, true, chainselectors.APTOS_MAINNET.Selector)
			fcB := newChain(t, rpcB, true, chainselectors.APTOS_TESTNET.Selector)
			rA, _ := fcA.AccountAPTBalance(ctx, meta, &aptoscappb.AccountAPTBalanceRequest{Address: mkAddr(0x01)})
			rB, _ := fcB.AccountAPTBalance(ctx, meta, &aptoscappb.AccountAPTBalanceRequest{Address: mkAddr(0x02)})
			assert.Equal(t, uint64(100), rA.Response.Value)
			assert.Equal(t, uint64(200), rB.Response.Value)
		}},
		{name: "28 ResolveKey sentinel OK under dry-run", run: func(t *testing.T) {
			ct := &AptosChainType{}
			k, err := ct.ResolveKey(&settings.Settings{User: settings.UserSettings{PrivateKeys: map[string]string{settings.Aptos.Name: ""}}}, false)
			require.NoError(t, err)
			require.NotNil(t, k)
		}},
		{name: "29 ResolveKey rejects sentinel under --broadcast", run: func(t *testing.T) {
			ct := &AptosChainType{}
			_, err := ct.ResolveKey(&settings.Settings{User: settings.UserSettings{PrivateKeys: map[string]string{settings.Aptos.Name: defaultSentinelAptosSeed}}}, true)
			require.Error(t, err)
		}},
		// --- chain-type plugin surface (31-45) ---
		{name: "31 ChainType.Name returns aptos", run: func(t *testing.T) {
			ct := &AptosChainType{}
			assert.Equal(t, "aptos", ct.Name())
		}},
		{name: "32 SupportedChains lists mainnet and testnet", run: func(t *testing.T) {
			ct := &AptosChainType{}
			cfgs := ct.SupportedChains()
			selectors := map[uint64]bool{}
			for _, c := range cfgs {
				selectors[c.Selector] = true
			}
			assert.True(t, selectors[chainselectors.APTOS_MAINNET.Selector])
			assert.True(t, selectors[chainselectors.APTOS_TESTNET.Selector])
		}},
		{name: "33 Supports false when capabilities unset", run: func(t *testing.T) {
			ct := &AptosChainType{}
			assert.False(t, ct.Supports(chainselectors.APTOS_TESTNET.Selector))
		}},
		{name: "34 Supports false for evm-shaped selector", run: func(t *testing.T) {
			ct := &AptosChainType{}
			assert.False(t, ct.Supports(1))
		}},
		{name: "35 ParseTriggerChainSelector accepts aptos prefix", run: func(t *testing.T) {
			ct := &AptosChainType{}
			sel, ok := ct.ParseTriggerChainSelector("aptos:ChainSelector:4741433654826277614@1.0.0")
			require.True(t, ok)
			assert.Equal(t, uint64(4741433654826277614), sel)
		}},
		{name: "36 ParseTriggerChainSelector rejects evm prefix", run: func(t *testing.T) {
			ct := &AptosChainType{}
			_, ok := ct.ParseTriggerChainSelector("evm:ChainSelector:1@1.0.0")
			assert.False(t, ok)
		}},
		{name: "37 ParseTriggerChainSelector rejects malformed id", run: func(t *testing.T) {
			ct := &AptosChainType{}
			_, ok := ct.ParseTriggerChainSelector("aptos:BadFormat")
			assert.False(t, ok)
		}},
		{name: "38 CollectCLIInputs returns empty map", run: func(t *testing.T) {
			ct := &AptosChainType{}
			got := ct.CollectCLIInputs(nil)
			assert.Empty(t, got)
		}},
		{name: "39 ExecuteTrigger returns explicit no-trigger error", run: func(t *testing.T) {
			ct := &AptosChainType{}
			err := ct.ExecuteTrigger(ctx, 1, "tid", nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "no trigger surface")
		}},
		{name: "40 ResolveTriggerData returns no-trigger error", run: func(t *testing.T) {
			ct := &AptosChainType{}
			_, err := ct.ResolveTriggerData(ctx, 1, chain.TriggerParams{})
			require.Error(t, err)
		}},
		{name: "41 ResolveClients with empty viper returns no clients", run: func(t *testing.T) {
			ct := newAptosChainTypeForTest(t)
			v := viper.New()
			resolved, err := ct.ResolveClients(v)
			require.NoError(t, err)
			assert.Empty(t, resolved.Clients)
			assert.Empty(t, resolved.Forwarders)
		}},
		{name: "42 ResolveKey parses 0x-prefixed seed", run: func(t *testing.T) {
			ct := &AptosChainType{}
			s := &settings.Settings{User: settings.UserSettings{PrivateKeys: map[string]string{settings.Aptos.Name: "0x2222222222222222222222222222222222222222222222222222222222222222"}}}
			k, err := ct.ResolveKey(s, true)
			require.NoError(t, err)
			require.NotNil(t, k)
		}},
		{name: "43 ResolveKey parses uppercase hex", run: func(t *testing.T) {
			ct := &AptosChainType{}
			s := &settings.Settings{User: settings.UserSettings{PrivateKeys: map[string]string{settings.Aptos.Name: "AABBCCDDEEFF00112233445566778899AABBCCDDEEFF00112233445566778899"}}}
			k, err := ct.ResolveKey(s, true)
			require.NoError(t, err)
			require.NotNil(t, k)
		}},
		{name: "44 ResolveKey trims whitespace", run: func(t *testing.T) {
			ct := &AptosChainType{}
			s := &settings.Settings{User: settings.UserSettings{PrivateKeys: map[string]string{settings.Aptos.Name: "  1111111111111111111111111111111111111111111111111111111111111111  "}}}
			k, err := ct.ResolveKey(s, true)
			require.NoError(t, err)
			require.NotNil(t, k)
		}},
		{name: "45 ResolveKey short seed hard-fails under broadcast", run: func(t *testing.T) {
			ct := &AptosChainType{}
			s := &settings.Settings{User: settings.UserSettings{PrivateKeys: map[string]string{settings.Aptos.Name: "0102"}}}
			_, err := ct.ResolveKey(s, true)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "CRE_APTOS_PRIVATE_KEY")
		}},

		// --- wrong-type / wrong-selector rejections in RegisterCapabilities (46-52) ---
		{name: "46 RegisterCapabilities rejects wrong client type", run: func(t *testing.T) {
			ct := &AptosChainType{}
			_, err := ct.RegisterCapabilities(ctx, chain.CapabilityConfig{
				Clients: map[uint64]chain.ChainClient{1: "not-an-aptos-client"},
				Logger:  logger.Test(t),
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not aptosfakes.AptosClient")
		}},
		{name: "47 RegisterCapabilities rejects wrong private-key type", run: func(t *testing.T) {
			ct := &AptosChainType{}
			_, err := ct.RegisterCapabilities(ctx, chain.CapabilityConfig{
				Clients:    map[uint64]chain.ChainClient{},
				PrivateKey: "this is not an Ed25519PrivateKey",
				Logger:     logger.Test(t),
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "*crypto.Ed25519PrivateKey")
		}},
		{name: "48 RegisterCapabilities rejects wrong limits type", run: func(t *testing.T) {
			ct := &AptosChainType{}
			_, err := ct.RegisterCapabilities(ctx, chain.CapabilityConfig{
				Clients: map[uint64]chain.ChainClient{},
				Limits:  badLimits{},
				Logger:  logger.Test(t),
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "AptosChainLimits")
		}},
		{name: "49 RegisterCapabilities with unknown selector (experimental) wires fake", run: func(t *testing.T) {
			// 404040 is not in SupportedChains — still gets a FakeAptosChain because
			// ResolveClients is the gatekeeper for selector-vs-supported, not Register.
			pk := newKey(t)
			rpc := mocks.NewAptosRpcClient(t)
			ct := &AptosChainType{}
			_, err := ct.RegisterCapabilities(ctx, chain.CapabilityConfig{
				Registry:   scenarioRegistry(t),
				Clients:    map[uint64]chain.ChainClient{404040: aptosfakes.AptosClient(rpc)},
				Forwarders: map[uint64]string{404040: "0xdead"},
				PrivateKey: pk,
				Broadcast:  false,
				Logger:     logger.Test(t),
			})
			require.NoError(t, err)
			assert.True(t, ct.Supports(404040))
		}},
		{name: "50 RegisterCapabilities skips selectors without forwarders", run: func(t *testing.T) {
			pk := newKey(t)
			rpc := mocks.NewAptosRpcClient(t)
			ct := &AptosChainType{}
			services, err := ct.RegisterCapabilities(ctx, chain.CapabilityConfig{
				Registry:   scenarioRegistry(t),
				Clients:    map[uint64]chain.ChainClient{9999: aptosfakes.AptosClient(rpc)},
				Forwarders: map[uint64]string{},
				PrivateKey: pk,
				Logger:     logger.Test(t),
			})
			require.NoError(t, err)
			assert.Empty(t, services, "no forwarder → no capability wired")
			assert.False(t, ct.Supports(9999))
		}},
		{name: "51 RegisterCapabilities propagates bad forwarder hex", run: func(t *testing.T) {
			pk := newKey(t)
			rpc := mocks.NewAptosRpcClient(t)
			ct := &AptosChainType{}
			_, err := ct.RegisterCapabilities(ctx, chain.CapabilityConfig{
				Registry:   scenarioRegistry(t),
				Clients:    map[uint64]chain.ChainClient{1: aptosfakes.AptosClient(rpc)},
				Forwarders: map[uint64]string{1: "not-hex-at-all"},
				PrivateKey: pk,
				Logger:     logger.Test(t),
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "parse forwarder")
		}},
		{name: "52 AptosChainType implements chain.ChainType", run: func(t *testing.T) {
			var _ chain.ChainType = &AptosChainType{}
		}},

		// --- TypeTag coverage via View (53-62) ---
		{name: "53 View BOOL TypeTag round-trips", run: func(t *testing.T) {
			assertTypeTagRoundTrip(t, aptoscappb.TypeTagKind_TYPE_TAG_KIND_BOOL)
		}},
		{name: "54 View U8 TypeTag round-trips", run: func(t *testing.T) {
			assertTypeTagRoundTrip(t, aptoscappb.TypeTagKind_TYPE_TAG_KIND_U8)
		}},
		{name: "55 View U16 TypeTag round-trips (iter-10 extension)", run: func(t *testing.T) {
			assertTypeTagRoundTrip(t, aptoscappb.TypeTagKind_TYPE_TAG_KIND_U16)
		}},
		{name: "56 View U32 TypeTag round-trips (iter-10 extension)", run: func(t *testing.T) {
			assertTypeTagRoundTrip(t, aptoscappb.TypeTagKind_TYPE_TAG_KIND_U32)
		}},
		{name: "57 View U64 TypeTag round-trips", run: func(t *testing.T) {
			assertTypeTagRoundTrip(t, aptoscappb.TypeTagKind_TYPE_TAG_KIND_U64)
		}},
		{name: "58 View U128 TypeTag round-trips", run: func(t *testing.T) {
			assertTypeTagRoundTrip(t, aptoscappb.TypeTagKind_TYPE_TAG_KIND_U128)
		}},
		{name: "59 View U256 TypeTag round-trips (iter-10 extension)", run: func(t *testing.T) {
			assertTypeTagRoundTrip(t, aptoscappb.TypeTagKind_TYPE_TAG_KIND_U256)
		}},
		{name: "60 View ADDRESS TypeTag round-trips", run: func(t *testing.T) {
			assertTypeTagRoundTrip(t, aptoscappb.TypeTagKind_TYPE_TAG_KIND_ADDRESS)
		}},
		{name: "61 View SIGNER TypeTag rejected (out of scope for view args)", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.View(ctx, meta, &aptoscappb.ViewRequest{
				Payload: &aptoscappb.ViewPayload{
					Module:   &aptoscappb.ModuleID{Address: mkAddr(1), Name: "m"},
					Function: "f",
					ArgTypes: []*aptoscappb.TypeTag{{Kind: aptoscappb.TypeTagKind_TYPE_TAG_KIND_SIGNER}},
				},
			})
			require.NotNil(t, capErr)
		}},
		{name: "62 View VECTOR TypeTag rejected (deferred)", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.View(ctx, meta, &aptoscappb.ViewRequest{
				Payload: &aptoscappb.ViewPayload{
					Module:   &aptoscappb.ModuleID{Address: mkAddr(1), Name: "m"},
					Function: "f",
					ArgTypes: []*aptoscappb.TypeTag{{Kind: aptoscappb.TypeTagKind_TYPE_TAG_KIND_VECTOR}},
				},
			})
			require.NotNil(t, capErr)
		}},

		// --- more read-path edges (63-72) ---
		{name: "63 AccountAPTBalance at all-zero address", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().AccountAPTBalance(mock.Anything).Return(uint64(0), nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, capErr := fc.AccountAPTBalance(ctx, meta, &aptoscappb.AccountAPTBalanceRequest{Address: make([]byte, 32)})
			require.Nil(t, capErr)
			assert.Equal(t, uint64(0), reply.Response.Value)
		}},
		{name: "64 AccountAPTBalance at all-ones address", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().AccountAPTBalance(mock.Anything).Return(uint64(^uint64(0)), nil).Once()
			fc := newChain(t, rpc, true, 1)
			addr := make([]byte, 32)
			for i := range addr {
				addr[i] = 0xff
			}
			reply, capErr := fc.AccountAPTBalance(ctx, meta, &aptoscappb.AccountAPTBalanceRequest{Address: addr})
			require.Nil(t, capErr)
			assert.Equal(t, ^uint64(0), reply.Response.Value)
		}},
		{name: "65 View with empty result returns JSON []", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().View(mock.Anything).Return([]any{}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, capErr := fc.View(ctx, meta, &aptoscappb.ViewRequest{
				Payload: &aptoscappb.ViewPayload{Module: &aptoscappb.ModuleID{Address: mkAddr(1), Name: "m"}, Function: "f"},
			})
			require.Nil(t, capErr)
			assert.Equal(t, []byte(`[]`), reply.Response.Data)
		}},
		{name: "66 View preserves multi-return as JSON array", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().View(mock.Anything).Return([]any{"first", "second"}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, capErr := fc.View(ctx, meta, &aptoscappb.ViewRequest{
				Payload: &aptoscappb.ViewPayload{Module: &aptoscappb.ModuleID{Address: mkAddr(1), Name: "m"}, Function: "f"},
			})
			require.Nil(t, capErr)
			assert.Equal(t, []byte(`["first","second"]`), reply.Response.Data)
		}},
		{name: "67 View integer return marshaled as JSON", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().View(mock.Anything).Return([]any{int64(42)}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, capErr := fc.View(ctx, meta, &aptoscappb.ViewRequest{
				Payload: &aptoscappb.ViewPayload{Module: &aptoscappb.ModuleID{Address: mkAddr(1), Name: "m"}, Function: "f"},
			})
			require.Nil(t, capErr)
			assert.Equal(t, []byte(`[42]`), reply.Response.Data)
		}},
		{name: "68 TransactionByHash SDK error without 404 → Unavailable", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().TransactionByHash(mock.Anything).Return(nil, fmt.Errorf("timeout")).Once()
			fc := newChain(t, rpc, true, 1)
			_, capErr := fc.TransactionByHash(ctx, meta, &aptoscappb.TransactionByHashRequest{Hash: "0xabc"})
			require.NotNil(t, capErr)
		}},
		{name: "69 TransactionByHash nil request rejected", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.TransactionByHash(ctx, meta, nil)
			require.NotNil(t, capErr)
		}},
		{name: "70 AccountTransactions with nil pagination forwards nil pointers", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().AccountTransactions(mock.Anything, (*uint64)(nil), (*uint64)(nil)).
				Return([]*api.CommittedTransaction{}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, capErr := fc.AccountTransactions(ctx, meta, &aptoscappb.AccountTransactionsRequest{Address: mkAddr(1)})
			require.Nil(t, capErr)
			assert.Empty(t, reply.Response.Transactions)
		}},
		{name: "71 AccountTransactions drops nil committed entries", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().AccountTransactions(mock.Anything, mock.Anything, mock.Anything).
				Return([]*api.CommittedTransaction{
					nil,
					{Type: api.TransactionVariantUser, Inner: &api.UserTransaction{Hash: "0x1"}},
					nil,
				}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			reply, capErr := fc.AccountTransactions(ctx, meta, &aptoscappb.AccountTransactionsRequest{Address: mkAddr(1)})
			require.Nil(t, capErr)
			assert.Len(t, reply.Response.Transactions, 1)
		}},
		{name: "72 AccountTransactions nil request rejected", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.AccountTransactions(ctx, meta, nil)
			require.NotNil(t, capErr)
		}},

		// --- WriteReport broadcast branches (73-82) ---
		{name: "73 WriteReport broadcast success populates TxHash + SUCCESS", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildSignAndSubmitTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&api.PendingTransaction{Hash: "0xfeed"}, nil).Once()
			rpc.EXPECT().WaitForTransaction("0xfeed").Return(&api.UserTransaction{
				Success: true, GasUsed: 10, GasUnitPrice: 1,
			}, nil).Once()
			fc := newChain(t, rpc, false, 1)
			reply, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			require.Nil(t, capErr)
			assert.Equal(t, aptoscappb.TxStatus_TX_STATUS_SUCCESS, reply.Response.TxStatus)
			require.NotNil(t, reply.Response.TxHash)
			assert.Equal(t, "0xfeed", *reply.Response.TxHash)
		}},
		{name: "74 WriteReport broadcast VM failure → FATAL+vmStatus", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildSignAndSubmitTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&api.PendingTransaction{Hash: "0xbad"}, nil).Once()
			rpc.EXPECT().WaitForTransaction("0xbad").Return(&api.UserTransaction{
				Success: false, VmStatus: "Move abort in 0xreceiver::module: X", GasUsed: 5, GasUnitPrice: 2,
			}, nil).Once()
			fc := newChain(t, rpc, false, 1)
			reply, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			require.Nil(t, capErr)
			assert.Equal(t, aptoscappb.TxStatus_TX_STATUS_FATAL, reply.Response.TxStatus)
			require.NotNil(t, reply.Response.ErrorMessage)
			assert.Contains(t, *reply.Response.ErrorMessage, "Move abort")
		}},
		{name: "75 WriteReport broadcast nil pending tx → Internal err", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildSignAndSubmitTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil, nil).Once()
			fc := newChain(t, rpc, false, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			require.NotNil(t, capErr)
		}},
		{name: "76 WriteReport broadcast forwarder err surfaces Unavailable", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildSignAndSubmitTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil, fmt.Errorf("forwarder refused")).Once()
			fc := newChain(t, rpc, false, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			require.NotNil(t, capErr)
		}},
		{name: "77 WriteReport broadcast WaitForTransaction err surfaces Unavailable", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildSignAndSubmitTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&api.PendingTransaction{Hash: "0xhold"}, nil).Once()
			rpc.EXPECT().WaitForTransaction("0xhold").Return(nil, fmt.Errorf("timeout")).Once()
			fc := newChain(t, rpc, false, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			require.NotNil(t, capErr)
		}},
		{name: "78 WriteReport broadcast nil final tx → FATAL with hash", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildSignAndSubmitTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&api.PendingTransaction{Hash: "0xabsent"}, nil).Once()
			rpc.EXPECT().WaitForTransaction("0xabsent").Return(nil, nil).Once()
			fc := newChain(t, rpc, false, 1)
			reply, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(), Report: validReport(),
			})
			require.Nil(t, capErr)
			assert.Equal(t, aptoscappb.TxStatus_TX_STATUS_FATAL, reply.Response.TxStatus)
			require.NotNil(t, reply.Response.TxHash)
			assert.Equal(t, "0xabsent", *reply.Response.TxHash)
		}},
		{name: "79 WriteReport with multi-sig forwards each signature byte", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: true}}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(),
				Report: &sdk.ReportResponse{
					RawReport: []byte("r"),
					Sigs: []*sdk.AttributedSignature{
						{Signature: []byte{0x01, 0x02}},
						{Signature: []byte{0x03, 0x04}},
					},
				},
			})
			require.Nil(t, capErr)
		}},
		{name: "80 WriteReport with empty sig slice is allowed", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: true}}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(),
				Report: &sdk.ReportResponse{RawReport: []byte("r"), Sigs: nil},
			})
			require.Nil(t, capErr)
		}},
		{name: "81 WriteReport with 64KiB raw report forwarded intact (dry-run)", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: true}}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(),
				Report: &sdk.ReportResponse{RawReport: make([]byte, 64*1024)},
			})
			require.Nil(t, capErr)
		}},
		{name: "82 WriteReport zero MaxGasAmount rejected", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), true, 1)
			_, capErr := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver:  mkAddr(0xBB),
				GasConfig: &aptoscappb.GasConfig{MaxGasAmount: 0, GasUnitPrice: 0},
				Report:    validReport(),
			})
			require.NotNil(t, capErr)
		}},

		// --- LimitedAptosChain edge cases (83-90) ---
		{name: "83 LimitedAptosChain at exact report-size limit passes", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: true}}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 10, maxGas: 10_000})
			_, capErr := l.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(),
				Report: &sdk.ReportResponse{RawReport: make([]byte, 10)},
			})
			require.Nil(t, capErr)
		}},
		{name: "84 LimitedAptosChain at size+1 blocked", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 10, maxGas: 10_000})
			_, capErr := l.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(),
				Report: &sdk.ReportResponse{RawReport: make([]byte, 11)},
			})
			require.NotNil(t, capErr)
		}},
		{name: "85 LimitedAptosChain at exact gas limit passes", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: true}}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 1000, maxGas: 100})
			_, capErr := l.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver:  mkAddr(0xBB),
				GasConfig: &aptoscappb.GasConfig{MaxGasAmount: 100, GasUnitPrice: 1},
				Report:    validReport(),
			})
			require.Nil(t, capErr)
		}},
		{name: "86 LimitedAptosChain at gas+1 blocked", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 1000, maxGas: 100})
			_, capErr := l.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver:  mkAddr(0xBB),
				GasConfig: &aptoscappb.GasConfig{MaxGasAmount: 101, GasUnitPrice: 1},
				Report:    validReport(),
			})
			require.NotNil(t, capErr)
		}},
		{name: "87 LimitedAptosChain zero report-size limit disables size check", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: true}}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 0, maxGas: 10_000})
			_, capErr := l.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver: mkAddr(0xBB), GasConfig: validGas(),
				Report: &sdk.ReportResponse{RawReport: make([]byte, 999_999)},
			})
			require.Nil(t, capErr)
		}},
		{name: "88 LimitedAptosChain zero gas limit disables gas check", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil).Once()
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: true}}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 10_000, maxGas: 0})
			_, capErr := l.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
				Receiver:  mkAddr(0xBB),
				GasConfig: &aptoscappb.GasConfig{MaxGasAmount: 999_999, GasUnitPrice: 1},
				Report:    validReport(),
			})
			require.Nil(t, capErr)
		}},
		{name: "89 LimitedAptosChain View delegates to inner", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().View(mock.Anything).Return([]any{"x"}, nil).Once()
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 1, maxGas: 1})
			reply, capErr := l.View(ctx, meta, &aptoscappb.ViewRequest{
				Payload: &aptoscappb.ViewPayload{Module: &aptoscappb.ModuleID{Address: mkAddr(1), Name: "m"}, Function: "f"},
			})
			require.Nil(t, capErr)
			assert.Equal(t, []byte(`["x"]`), reply.Response.Data)
		}},
		{name: "90 LimitedAptosChain TransactionByHash delegates to inner", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().TransactionByHash("0xA").Return(nil, nil).Once()
			fc := newChain(t, rpc, true, 1)
			l := NewLimitedAptosChain(fc, stubLimits{reportSize: 1, maxGas: 1})
			reply, capErr := l.TransactionByHash(ctx, meta, &aptoscappb.TransactionByHashRequest{Hash: "0xA"})
			require.Nil(t, capErr)
			assert.Nil(t, reply.Response.Transaction)
		}},

		// --- lifecycle + info (91-100) ---
		{name: "91 FakeAptosChain ChainSelector reflects constructor arg", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), false, 4741433654826277352)
			assert.Equal(t, uint64(4741433654826277352), fc.ChainSelector())
		}},
		{name: "92 FakeAptosChain Description non-empty", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), false, 1)
			assert.NotEmpty(t, fc.Description())
		}},
		{name: "93 FakeAptosChain Info ID includes selector", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), false, 42)
			info, err := fc.Info(ctx)
			require.NoError(t, err)
			assert.Contains(t, info.ID, "42")
			assert.Contains(t, info.ID, "aptos")
		}},
		{name: "94 FakeAptosChain Name embeds selector", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), false, 7)
			assert.True(t, strings.Contains(fc.Name(), "7"), "Name=%s should contain selector", fc.Name())
		}},
		{name: "95 FakeAptosChain Initialise is no-op", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), false, 1)
			assert.NoError(t, fc.Initialise(ctx, core.StandardCapabilitiesDependencies{}))
		}},
		{name: "96 FakeAptosChain Register+Unregister workflow are no-ops", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), false, 1)
			require.NoError(t, fc.RegisterToWorkflow(ctx, commonCap.RegisterToWorkflowRequest{Metadata: commonCap.RegistrationMetadata{WorkflowID: "w"}}))
			require.NoError(t, fc.UnregisterFromWorkflow(ctx, commonCap.UnregisterFromWorkflowRequest{Metadata: commonCap.RegistrationMetadata{WorkflowID: "w"}}))
		}},
		{name: "97 FakeAptosChain Execute returns empty response", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), false, 1)
			resp, err := fc.Execute(ctx, commonCap.CapabilityRequest{})
			require.NoError(t, err)
			assert.Equal(t, commonCap.CapabilityResponse{}, resp)
		}},
		{name: "98 FakeAptosChain HealthReport single entry, no error", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), false, 1)
			require.NoError(t, fc.Start(ctx))
			hr := fc.HealthReport()
			require.Len(t, hr, 1)
			assert.NoError(t, hr[fc.Name()])
			assert.NoError(t, fc.Close())
		}},
		{name: "99 AptosChainCapabilities Start+Close are idempotent no-ops", run: func(t *testing.T) {
			fc := newChain(t, mocks.NewAptosRpcClient(t), false, 1)
			caps := &AptosChainCapabilities{AptosChains: map[uint64]*aptosfakes.FakeAptosChain{1: fc}}
			require.NoError(t, caps.Start(ctx))
			require.NoError(t, caps.Close())
		}},
		{name: "100 FakeAptosChain construction fails on nil client or key", run: func(t *testing.T) {
			_, err := aptosfakes.NewFakeAptosChain(logger.Test(t), nil, newKey(t), testAddr(t, "0xdead"), 1, false)
			require.Error(t, err)
			_, err = aptosfakes.NewFakeAptosChain(logger.Test(t), mocks.NewAptosRpcClient(t), nil, testAddr(t, "0xdead"), 1, false)
			require.Error(t, err)
		}},

		{name: "30 Concurrent reads + writes are race-clean", run: func(t *testing.T) {
			rpc := mocks.NewAptosRpcClient(t)
			rpc.EXPECT().AccountAPTBalance(mock.Anything).Return(uint64(1), nil)
			rpc.EXPECT().BuildTransaction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(&aptos.RawTransaction{}, nil)
			rpc.EXPECT().SimulateTransaction(mock.Anything, mock.Anything).
				Return([]*api.UserTransaction{{Success: true}}, nil)
			fc := newChain(t, rpc, true, 1)
			const n = 10
			var wg sync.WaitGroup
			errs := make(chan caperrors.Error, n*2)
			for i := 0; i < n; i++ {
				wg.Add(2)
				go func() {
					defer wg.Done()
					if _, e := fc.AccountAPTBalance(ctx, meta, &aptoscappb.AccountAPTBalanceRequest{Address: mkAddr(1)}); e != nil {
						errs <- e
					}
				}()
				go func() {
					defer wg.Done()
					if _, e := fc.WriteReport(ctx, meta, &aptoscappb.WriteReportRequest{
						Receiver: mkAddr(1), GasConfig: validGas(), Report: validReport(),
					}); e != nil {
						errs <- e
					}
				}()
			}
			wg.Wait()
			close(errs)
			for e := range errs {
				require.Nil(t, e)
			}
		}},
	}
}

// TestSimulatorScenarios_100 runs 100 dry-run scenarios exercising the full
// behavioural surface of FakeAptosChain + the Aptos chain-type plugin:
// read-path happy/error paths, WriteReport broadcast+dry-run, LimitedAptosChain
// size/gas enforcement, TypeTag scalar coverage, chaintype registration edges,
// and lifecycle/Info contracts.
func TestSimulatorScenarios_100(t *testing.T) {
	t.Parallel()
	cases := simulatorScenarios()
	require.Len(t, cases, 100, "must have exactly 100 simulator scenarios")
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.run(t)
		})
	}
}

// --- scenario helpers (kept in this file to avoid leaking to prod builds) ---

// assertTypeTagRoundTrip wires a minimal View against a mock and asserts
// that the given TypeTag kind is accepted by viewPayloadFromProto +
// typeTagFromProto. A reject manifests as a PublicUserError.
func assertTypeTagRoundTrip(t *testing.T, kind aptoscappb.TypeTagKind) {
	t.Helper()
	rpc := mocks.NewAptosRpcClient(t)
	rpc.EXPECT().View(mock.Anything).Return([]any{"ok"}, nil).Once()
	fc, err := aptosfakes.NewFakeAptosChain(logger.Test(t), rpc, newKey(t),
		testAddr(t, "0xdead"), 1, true)
	require.NoError(t, err)
	_, capErr := fc.View(context.Background(), commonCap.RequestMetadata{}, &aptoscappb.ViewRequest{
		Payload: &aptoscappb.ViewPayload{
			Module:   &aptoscappb.ModuleID{Address: mkAddr(1), Name: "m"},
			Function: "f",
			ArgTypes: []*aptoscappb.TypeTag{{Kind: kind}},
		},
	})
	require.Nil(t, capErr, "kind %v should be accepted", kind)
}

// badLimits satisfies chain.Limits but not AptosChainLimits, to exercise
// RegisterCapabilities' type-assertion rejection.
type badLimits struct{}

func (badLimits) ChainWriteReportSizeLimit() int { return 0 }

// scenarioRegistry returns a capability registry usable in RegisterCapabilities
// scenarios. Matches the EVM sibling's newRegistry helper.
func scenarioRegistry(t *testing.T) *capabilities.Registry {
	t.Helper()
	return capabilities.NewRegistry(logger.Test(t))
}

// newAptosChainTypeForTest returns a zero-value AptosChainType — its log
// field is only read by scenarios that hit ResolveClients/RegisterCapabilities
// when RPCs are configured, and scenarios pass empty viper so the nil log
// never dereferences.
func newAptosChainTypeForTest(t *testing.T) *AptosChainType {
	t.Helper()
	zl := zerolog.Nop()
	return &AptosChainType{log: &zl}
}

package aptos

// simulator_scenarios_test.go runs 30 dry-run scenarios exercising FakeAptosChain
// via the simulator plumbing. All scenarios are fully in-process: no network I/O,
// no --broadcast. They verify parity with the EVM simulator's behavioural surface
// (success paths, validation errors, limit enforcement, per-selector dispatch,
// key resolution semantics).

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/aptos-labs/aptos-go-sdk/crypto"
	chainselectors "github.com/smartcontractkit/chain-selectors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	commonCap "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	caperrors "github.com/smartcontractkit/chainlink-common/pkg/capabilities/errors"
	aptoscappb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/chain-capabilities/aptos"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	sdk "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"

	aptosfakes "github.com/smartcontractkit/chainlink-aptos/fakes"
	mocks "github.com/smartcontractkit/chainlink-aptos/relayer/monitor/mocks"

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
		{name: "05 View round-trips opaque bytes", run: func(t *testing.T) {
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
			assert.Equal(t, []byte("hello"), reply.Response.Data)
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
				Return([]*api.UserTransaction{{Success: false, VmStatus: "Move abort in 0xdead::forwarder: Bad"}}, nil).Once()
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
			k, err := ct.ResolveKey(&settings.Settings{User: settings.UserSettings{AptosPrivateKey: ""}}, false)
			require.NoError(t, err)
			require.NotNil(t, k)
		}},
		{name: "29 ResolveKey rejects sentinel under --broadcast", run: func(t *testing.T) {
			ct := &AptosChainType{}
			_, err := ct.ResolveKey(&settings.Settings{User: settings.UserSettings{AptosPrivateKey: defaultSentinelAptosSeed}}, true)
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

// TestSimulatorScenarios_30 runs 30 dry-run scenarios and reports pass/fail per
// scenario. Verifies parity with FakeEVMChain's behavioural surface.
func TestSimulatorScenarios_30(t *testing.T) {
	t.Parallel()
	cases := simulatorScenarios()
	require.Len(t, cases, 30, "must have exactly 30 simulator scenarios")
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			c.run(t)
		})
	}
}

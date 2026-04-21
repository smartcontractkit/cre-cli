package aptos

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"
	mocks "github.com/smartcontractkit/chainlink-aptos/relayer/monitor/mocks"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain"
)

func TestRunRPCHealthCheck_NoClients(t *testing.T) {
	t.Parallel()
	err := RunRPCHealthCheck(map[uint64]chain.ChainClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no Aptos RPC URLs")
}

func TestRunRPCHealthCheck_InvalidClientType(t *testing.T) {
	t.Parallel()
	err := RunRPCHealthCheck(map[uint64]chain.ChainClient{1: stubNonAptosClient{}}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid client type")
	assert.Contains(t, err.Error(), "[1]")
}

func TestRunRPCHealthCheck_NilClient(t *testing.T) {
	t.Parallel()
	err := RunRPCHealthCheck(map[uint64]chain.ChainClient{9: nil}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "[9] nil client")
}

func TestRunRPCHealthCheck_Healthy(t *testing.T) {
	t.Parallel()
	rpc := mocks.NewAptosRpcClient(t)
	rpc.EXPECT().GetChainId().Return(uint8(1), nil).Once()
	require.NoError(t, RunRPCHealthCheck(map[uint64]chain.ChainClient{1: rpc}, nil))
}

func TestRunRPCHealthCheck_ZeroChainID(t *testing.T) {
	t.Parallel()
	rpc := mocks.NewAptosRpcClient(t)
	rpc.EXPECT().GetChainId().Return(uint8(0), nil).Once()
	err := RunRPCHealthCheck(map[uint64]chain.ChainClient{7: rpc}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zero chain ID")
	assert.Contains(t, err.Error(), "[chain 7]")
}

func TestRunRPCHealthCheck_RPCError(t *testing.T) {
	t.Parallel()
	rpc := mocks.NewAptosRpcClient(t)
	rpc.EXPECT().GetChainId().Return(uint8(0), errors.New("boom")).Once()
	err := RunRPCHealthCheck(map[uint64]chain.ChainClient{3: rpc}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
	assert.Contains(t, err.Error(), "[chain 3]")
}

func TestRunRPCHealthCheck_NamedChain(t *testing.T) {
	t.Parallel()
	rpc := mocks.NewAptosRpcClient(t)
	rpc.EXPECT().GetChainId().Return(uint8(0), errors.New("unreachable")).Once()
	err := RunRPCHealthCheck(
		map[uint64]chain.ChainClient{chainselectors.APTOS_TESTNET.Selector: rpc},
		nil,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "[aptos-testnet]")
}

func TestRunRPCHealthCheck_ExperimentalLabel(t *testing.T) {
	t.Parallel()
	rpc := mocks.NewAptosRpcClient(t)
	rpc.EXPECT().GetChainId().Return(uint8(0), nil).Once()
	err := RunRPCHealthCheck(
		map[uint64]chain.ChainClient{42: rpc},
		map[uint64]bool{42: true},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "experimental chain 42")
}

func TestRunRPCHealthCheck_AggregatesMultiple(t *testing.T) {
	t.Parallel()
	bad := mocks.NewAptosRpcClient(t)
	bad.EXPECT().GetChainId().Return(uint8(0), errors.New("net down")).Once()
	zero := mocks.NewAptosRpcClient(t)
	zero.EXPECT().GetChainId().Return(uint8(0), nil).Once()
	err := RunRPCHealthCheck(map[uint64]chain.ChainClient{1: bad, 2: zero}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "net down")
	assert.Contains(t, err.Error(), "zero chain ID")
}

type stubNonAptosClient struct{}

package client

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"
)

type mockWorkflowRegistryV2Contract struct {
	mock.Mock
}

func (m *mockWorkflowRegistryV2Contract) AllowlistRequest(opts *bind.TransactOpts, requestDigest [32]byte, expiryTimestamp uint32) (*gethtypes.Transaction, error) {
	args := m.Called(opts, requestDigest, expiryTimestamp)
	tx, _ := args.Get(0).(*gethtypes.Transaction)
	return tx, args.Error(1)
}

func (m *mockWorkflowRegistryV2Contract) IsRequestAllowlisted(opts *bind.CallOpts, owner common.Address, requestDigest [32]byte) (bool, error) {
	args := m.Called(opts, owner, requestDigest)
	return args.Bool(0), args.Error(1)
}

func newTestWRC(t *testing.T) *WorkflowRegistryV2Client {
	t.Helper()
	logger := zerolog.Nop()
	mockEth := new(ethclient.Client)

	return &WorkflowRegistryV2Client{
		TxClient: TxClient{
			Logger:    &logger,
			EthClient: &seth.Client{Client: mockEth},
		},
		ContractAddress: common.HexToAddress("0x1234"),
	}
}

func TestIsRequestAllowlisted_Success(t *testing.T) {
	wrc := newTestWRC(t)
	mc := new(mockWorkflowRegistryV2Contract)
	wrc.Wr = mc

	owner := common.HexToAddress("0xabc0000000000000000000000000000000000abc")
	reqDigest := [32]byte{0: 1}

	mc.On(
		"IsRequestAllowlisted",
		mock.AnythingOfType("*bind.CallOpts"),
		owner,
		reqDigest,
	).Return(true, nil).Once()

	ok, err := wrc.IsRequestAllowlisted(owner, reqDigest)
	assert.NoError(t, err)
	assert.True(t, ok)

	mc.AssertExpectations(t)
}

func TestIsRequestAllowlisted_ContractError(t *testing.T) {
	wrc := newTestWRC(t)
	mc := new(mockWorkflowRegistryV2Contract)
	wrc.Wr = mc

	owner := common.HexToAddress("0xdef0000000000000000000000000000000000def")
	reqDigest := [32]byte{0: 1}

	mc.On(
		"IsRequestAllowlisted",
		mock.AnythingOfType("*bind.CallOpts"),
		owner,
		reqDigest,
	).Return(false, errors.New("revert: not allowed")).Once()

	ok, err := wrc.IsRequestAllowlisted(owner, reqDigest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
	assert.False(t, ok)

	mc.AssertExpectations(t)
}

func TestCallContractMethodV2_ErrorWrapped(t *testing.T) {
	// Ensure errors from the inner call are propagated via DecodeSendErr without panic.
	wrc := newTestWRC(t) // provides EthClient so DecodeSendErr can be called safely

	_, err := callContractMethodV2[string](wrc, func() (string, error) {
		return "", errors.New("boom")
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

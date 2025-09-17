package client

import (
	"errors"
	"strings"
	"testing"
	"time"

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

func mustBytes32(t *testing.T, h string) [32]byte {
	t.Helper()
	out, err := HexToBytes32(h)
	if err != nil {
		t.Fatalf("HexToBytes32(%q) failed: %v", h, err)
	}
	return out
}

func TestIsRequestAllowlisted_Success(t *testing.T) {
	wrc := newTestWRC(t)
	mc := new(mockWorkflowRegistryV2Contract)
	wrc.Wr = mc

	owner := common.HexToAddress("0xabc0000000000000000000000000000000000abc")
	digestHex := "0x" + strings.Repeat("11", 32)
	reqDigest := mustBytes32(t, digestHex)

	mc.On(
		"IsRequestAllowlisted",
		mock.AnythingOfType("*bind.CallOpts"),
		owner,
		reqDigest,
	).Return(true, nil).Once()

	ok, err := wrc.IsRequestAllowlisted(owner, digestHex)
	assert.NoError(t, err)
	assert.True(t, ok)

	mc.AssertExpectations(t)
}

func TestIsRequestAllowlisted_ContractError(t *testing.T) {
	wrc := newTestWRC(t)
	mc := new(mockWorkflowRegistryV2Contract)
	wrc.Wr = mc

	owner := common.HexToAddress("0xdef0000000000000000000000000000000000def")
	digestHex := "0x" + strings.Repeat("22", 32)
	reqDigest := mustBytes32(t, digestHex)

	mc.On(
		"IsRequestAllowlisted",
		mock.AnythingOfType("*bind.CallOpts"),
		owner,
		reqDigest,
	).Return(false, errors.New("revert: not allowed")).Once()

	ok, err := wrc.IsRequestAllowlisted(owner, digestHex)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
	assert.False(t, ok)

	mc.AssertExpectations(t)
}

func TestIsRequestAllowlisted_InvalidDigest(t *testing.T) {
	wrc := newTestWRC(t)
	wrc.Wr = new(mockWorkflowRegistryV2Contract)

	// 31 bytes (too short)
	bad := "0x" + strings.Repeat("aa", 31)
	ok, err := wrc.IsRequestAllowlisted(common.HexToAddress("0x1"), bad)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "digest must be 32 bytes")
	assert.False(t, ok)
}

func TestAllowlistRequest_InvalidDigest(t *testing.T) {
	wrc := newTestWRC(t)
	wrc.Wr = new(mockWorkflowRegistryV2Contract)

	bad := "0x" + strings.Repeat("ff", 31) // 31 bytes
	err := wrc.AllowlistRequest(bad, 10*time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "digest must be 32 bytes")
}

func TestHexToBytes32_Success(t *testing.T) {
	h := "0x" + strings.Repeat("ab", 32) // exactly 32 bytes
	out, err := HexToBytes32(h)
	assert.NoError(t, err)
	// simple sanity check: first and last bytes match expected
	assert.Equal(t, byte(0xab), out[0])
	assert.Equal(t, byte(0xab), out[31])
}

func TestHexToBytes32_InvalidHex(t *testing.T) {
	_, err := HexToBytes32("0xzz") // invalid
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid hex")
}

func TestHexToBytes32_WrongLength(t *testing.T) {
	// 30 bytes
	_, err := HexToBytes32("0x" + strings.Repeat("aa", 30))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "digest must be 32 bytes")
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

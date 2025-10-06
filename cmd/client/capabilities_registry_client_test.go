package client

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/proto"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	capv2 "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"
	"github.com/smartcontractkit/chainlink-testing-framework/seth"
)

// MockCapabilitiesRegistry is a mock implementation of the new interface
// to be used for testing.
type MockCapabilitiesRegistry struct {
	mock.Mock
}

func (m *MockCapabilitiesRegistry) GetDONsInFamily(opts *bind.CallOpts, donFamily string, start *big.Int, limit *big.Int) ([]*big.Int, error) {
	args := m.Called(opts, donFamily)
	return args.Get(0).([]*big.Int), args.Error(1)
}

func (m *MockCapabilitiesRegistry) GetDON(opts *bind.CallOpts, donId uint32) (capv2.CapabilitiesRegistryDONInfo, error) {
	args := m.Called(opts, donId)
	return args.Get(0).(capv2.CapabilitiesRegistryDONInfo), args.Error(1)
}

func TestGetVaultMasterPublicKey(t *testing.T) {
	logger := zerolog.Nop()
	mockEthClient := new(ethclient.Client)
	mockSethClient := &seth.Client{
		Client: mockEthClient,
	}

	t.Run("success - returns master key config for existing DON family and vault capability", func(t *testing.T) {
		// Create the mock object
		mockCR := new(MockCapabilitiesRegistry)
		// Manually create the client struct and inject the mock
		client := &CapabilitiesRegistryClient{
			TxClient: TxClient{
				Logger:    &logger,
				EthClient: mockSethClient,
			},
			ContractAddress: common.HexToAddress("0x1234"),
			Cr:              mockCR,
		}

		mockCR.On("GetDONsInFamily", (*bind.CallOpts)(nil), "data-streams").Return([]*big.Int{big.NewInt(1)}, nil).Once()

		mockPublicKey := []byte("a_valid_mock_public_key_bytes")
		mockMap := &valuespb.Map{
			Fields: map[string]*valuespb.Value{
				"VaultPublicKey": {
					Value: &valuespb.Value_BytesValue{
						BytesValue: mockPublicKey,
					},
				},
			},
		}

		capConfig := &pb.CapabilityConfig{
			DefaultConfig: mockMap,
		}

		vaultConfigBytes, err := proto.Marshal(capConfig)
		assert.NoError(t, err)

		mockDonInfo := capv2.CapabilitiesRegistryDONInfo{
			CapabilityConfigurations: []capv2.CapabilitiesRegistryCapabilityConfiguration{
				{CapabilityId: vault.CapabilityID, Config: vaultConfigBytes},
			},
		}
		mockCR.On("GetDON", (*bind.CallOpts)(nil), uint32(1)).Return(mockDonInfo, nil).Once()

		result, err := client.GetVaultMasterPublicKey("data-streams")

		assert.NoError(t, err)
		assert.Equal(t, mockPublicKey, result)
		mockCR.AssertExpectations(t)
	})

	t.Run("failure - no DONs found for the provided DON family", func(t *testing.T) {
		mockCR := new(MockCapabilitiesRegistry)
		client := &CapabilitiesRegistryClient{
			TxClient: TxClient{
				Logger:    &logger,
				EthClient: mockSethClient,
			},
			ContractAddress: common.HexToAddress("0x1234"),
			Cr:              mockCR,
		}

		mockCR.On("GetDONsInFamily", (*bind.CallOpts)(nil), "non-existent-family").Return([]*big.Int{}, nil).Once()

		result, err := client.GetVaultMasterPublicKey("non-existent-family")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.EqualError(t, err, "no DONs found for the provided donFamily: non-existent-family")
		mockCR.AssertExpectations(t)
	})

	t.Run("failure - DONs found but no vault capability", func(t *testing.T) {
		mockCR := new(MockCapabilitiesRegistry)
		client := &CapabilitiesRegistryClient{
			TxClient: TxClient{
				Logger:    &logger,
				EthClient: mockSethClient,
			},
			ContractAddress: common.HexToAddress("0x1234"),
			Cr:              mockCR,
		}

		mockCR.On("GetDONsInFamily", (*bind.CallOpts)(nil), "data-streams").Return([]*big.Int{big.NewInt(1)}, nil).Once()
		mockDonInfo := capv2.CapabilitiesRegistryDONInfo{
			CapabilityConfigurations: []capv2.CapabilitiesRegistryCapabilityConfiguration{
				{CapabilityId: "other@1.0.0", Config: []byte("other-config")},
			},
		}
		mockCR.On("GetDON", (*bind.CallOpts)(nil), uint32(1)).Return(mockDonInfo, nil).Once()

		result, err := client.GetVaultMasterPublicKey("data-streams")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.EqualError(t, err, "no DON found with the required 'vault@1.0.0' capability")
		mockCR.AssertExpectations(t)
	})

	t.Run("failure - error from GetDONsInFamily", func(t *testing.T) {
		mockCR := new(MockCapabilitiesRegistry)
		client := &CapabilitiesRegistryClient{
			TxClient: TxClient{
				Logger:    &logger,
				EthClient: mockSethClient,
			},
			ContractAddress: common.HexToAddress("0x1234"),
			Cr:              mockCR,
		}

		expectedErr := errors.New("mock GetDONsInFamily error")
		mockCR.On("GetDONsInFamily", (*bind.CallOpts)(nil), "data-streams").Return([]*big.Int{}, expectedErr).Once()

		result, err := client.GetVaultMasterPublicKey("data-streams")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), expectedErr.Error())
		mockCR.AssertExpectations(t)
	})

	t.Run("failure - error from GetDON call for a specific DON", func(t *testing.T) {
		mockCR := new(MockCapabilitiesRegistry)
		client := &CapabilitiesRegistryClient{
			TxClient: TxClient{
				Logger:    &logger,
				EthClient: mockSethClient,
			},
			ContractAddress: common.HexToAddress("0x1234"),
			Cr:              mockCR,
		}

		mockCR.On("GetDONsInFamily", (*bind.CallOpts)(nil), "data-streams").Return([]*big.Int{big.NewInt(1)}, nil).Once()
		expectedErr := errors.New("mock GetDON error")
		mockCR.On("GetDON", (*bind.CallOpts)(nil), uint32(1)).Return(capv2.CapabilitiesRegistryDONInfo{}, expectedErr).Once()

		result, err := client.GetVaultMasterPublicKey("data-streams")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.EqualError(t, err, "no DON found with the required 'vault@1.0.0' capability")
		mockCR.AssertExpectations(t)
	})
}

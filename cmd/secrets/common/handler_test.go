package common

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/nacl/box"
	"google.golang.org/protobuf/proto"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	capv2 "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
	valuespb "github.com/smartcontractkit/chainlink-protos/cre/go/values/pb"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
)

var randReader = rand.Reader

type mockCapabilitiesRegistry struct {
	publicKey                    *[32]byte
	getVaultMasterPublicKeyError error
}

func (m *mockCapabilitiesRegistry) GetDONsInFamily(opts *bind.CallOpts, donFamily string) ([]*big.Int, error) {
	if donFamily != constants.DefaultStagingDonFamily {
		return nil, errors.New("invalid don family")
	}
	return []*big.Int{big.NewInt(1)}, nil
}

func (m *mockCapabilitiesRegistry) GetDON(opts *bind.CallOpts, donId uint32) (capv2.CapabilitiesRegistryDONInfo, error) {
	if donId != 1 {
		return capv2.CapabilitiesRegistryDONInfo{}, errors.New("invalid don id")
	}
	if m.getVaultMasterPublicKeyError != nil {
		return capv2.CapabilitiesRegistryDONInfo{}, m.getVaultMasterPublicKeyError
	}

	mockMap := &valuespb.Map{
		Fields: map[string]*valuespb.Value{
			"VaultPublicKey": {Value: &valuespb.Value_BytesValue{BytesValue: m.publicKey[:]}},
		},
	}
	capConfig := &pb.CapabilityConfig{DefaultConfig: mockMap}
	vaultConfigBytes, _ := proto.Marshal(capConfig)

	return capv2.CapabilitiesRegistryDONInfo{
		CapabilityConfigurations: []capv2.CapabilitiesRegistryCapabilityConfiguration{
			{CapabilityId: vault.CapabilityID, Config: vaultConfigBytes},
		},
	}, nil
}

func TestEncryptSecrets(t *testing.T) {
	h, mockClientFactory, _ := newMockHandler(t)

	t.Run("success - encrypts secrets with a valid public key", func(t *testing.T) {
		publicKey, privateKey, err := box.GenerateKey(randReader) // small shim for test-only rand
		assert.NoError(t, err)

		mockClientFactory.
			On("NewCapabilitiesRegistryClient").
			Return(&client.CapabilitiesRegistryClient{
				TxClient: client.TxClient{Logger: h.Log},
				Cr: &mockCapabilitiesRegistry{
					publicKey: publicKey,
				},
				ContractAddress: common.HexToAddress("0x1234"),
			}, nil).
			Once()

		raw := UpsertSecretsInputs{
			{ID: "test-secret-1", Value: "value1", Namespace: "ns1"},
			{ID: "test-secret-2", Value: "another-value", Namespace: "ns2"},
		}
		enc, err := h.EncryptSecrets(raw)
		assert.NoError(t, err)
		assert.Len(t, enc, 2)

		assert.Equal(t, "test-secret-1", enc[0].Id.Key)
		assert.Equal(t, "ns1", enc[0].Id.Namespace)
		assert.Equal(t, "0xabc", enc[0].Id.Owner)

		assert.Equal(t, "test-secret-2", enc[1].Id.Key)
		assert.Equal(t, "ns2", enc[1].Id.Namespace)
		assert.Equal(t, "0xabc", enc[1].Id.Owner)

		b1, _ := hex.DecodeString(enc[0].EncryptedValue)
		out1, ok := box.OpenAnonymous(nil, b1, publicKey, privateKey)
		assert.True(t, ok)
		assert.Equal(t, "value1", string(out1))

		b2, _ := hex.DecodeString(enc[1].EncryptedValue)
		out2, ok := box.OpenAnonymous(nil, b2, publicKey, privateKey)
		assert.True(t, ok)
		assert.Equal(t, "another-value", string(out2))
	})

	t.Run("failure - client creation error", func(t *testing.T) {
		mockClientFactory.
			On("NewCapabilitiesRegistryClient").
			Return(&client.CapabilitiesRegistryClient{}, errors.New("client creation error")).
			Once()

		enc, err := h.EncryptSecrets(UpsertSecretsInputs{{ID: "s", Value: "v", Namespace: "n"}})
		assert.Error(t, err)
		assert.Nil(t, enc)
		assert.Contains(t, err.Error(), "client creation error")
	})

	t.Run("failure - public key fetch error", func(t *testing.T) {
		mockClientFactory.
			On("NewCapabilitiesRegistryClient").
			Return(&client.CapabilitiesRegistryClient{
				TxClient: client.TxClient{Logger: h.Log},
				Cr: &mockCapabilitiesRegistry{
					getVaultMasterPublicKeyError: errors.New("pk error"),
				},
				ContractAddress: common.HexToAddress("0x1234"),
			}, nil).
			Once()

		enc, err := h.EncryptSecrets(UpsertSecretsInputs{{ID: "s", Value: "v", Namespace: "n"}})
		assert.Error(t, err)
		assert.Nil(t, enc)
		assert.Contains(t, err.Error(), "failed to fetch master public key")
	})
}

func TestPackAllowlistRequestTxData_Success_With0x(t *testing.T) {
	h, _, _ := newMockHandler(t)

	// random 32-byte digest
	var d [32]byte
	_, err := rand.Read(d[:])
	require.NoError(t, err)

	digestHex := "0x" + hex.EncodeToString(d[:])
	dur := 15 * time.Minute
	start := time.Now().Unix()

	// call
	dataHex, err := h.PackAllowlistRequestTxData(digestHex, dur)
	require.NoError(t, err)
	require.NotEmpty(t, dataHex)

	// decode and verify selector + args
	data, err := hex.DecodeString(dataHex)
	require.NoError(t, err)

	abiSpec, err := abi.JSON(strings.NewReader(workflow_registry_wrapper_v2.WorkflowRegistryMetaData.ABI))
	require.NoError(t, err)

	m := abiSpec.Methods["allowlistRequest"]
	require.NotNil(t, m)

	// first 4 bytes: function selector
	require.GreaterOrEqual(t, len(data), 4)
	assert.Equal(t, m.ID[:], data[:4], "method selector must match")

	// unpack args (skip selector)
	args, err := m.Inputs.Unpack(data[4:])
	require.NoError(t, err)
	require.Len(t, args, 2)

	// arg0: bytes32
	gotDigest, ok := args[0].([32]byte)
	require.True(t, ok)
	assert.Equal(t, d, gotDigest)

	// arg1: uint32 expiry ~ now + dur (allow small skew)
	gotExpiry, ok := args[1].(uint32)
	require.Truef(t, ok, "expiry should be uint32, got %T", args[1])

	exp := int64(gotExpiry) // seconds since epoch
	target := start + int64(dur/time.Second)

	assert.GreaterOrEqual(t, exp, target-5)
	assert.LessOrEqual(t, exp, target+60)
}

func TestPackAllowlistRequestTxData_Success_No0x(t *testing.T) {
	h, _, _ := newMockHandler(t)

	var d [32]byte
	_, err := rand.Read(d[:])
	require.NoError(t, err)

	digestHex := hex.EncodeToString(d[:]) // no 0x prefix
	dataHex, err := h.PackAllowlistRequestTxData(digestHex, 1*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, dataHex)
}

func TestPackAllowlistRequestTxData_InvalidDigest(t *testing.T) {
	h, _, _ := newMockHandler(t)

	// too short -> invalid
	_, err := h.PackAllowlistRequestTxData("0xdeadbeef", 10*time.Minute)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request digest")
}

package vaultdon_test

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	vaultcommon "github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	capabilitiespb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	capreg "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common/vaultdon"
	"github.com/smartcontractkit/cre-cli/internal/onchain/capabilitiesregistry"
)

type mockReader struct {
	donIDs []*big.Int
	dons   map[uint32]capreg.CapabilitiesRegistryDONInfo
	nodes  []capreg.INodeInfoProviderNodeInfo
}

func (m *mockReader) GetDONsInFamily(_ context.Context, _ string) ([]*big.Int, error) {
	return m.donIDs, nil
}

func (m *mockReader) GetDON(_ context.Context, donID uint32) (capreg.CapabilitiesRegistryDONInfo, error) {
	don, ok := m.dons[donID]
	if !ok {
		return capreg.CapabilitiesRegistryDONInfo{}, fmt.Errorf("DON %d not found", donID)
	}
	return don, nil
}

func (m *mockReader) GetNodes(_ context.Context) ([]capreg.INodeInfoProviderNodeInfo, error) {
	return m.nodes, nil
}

func vaultConfigBytes(t *testing.T, publicKey string, threshold int) []byte {
	t.Helper()
	valueMap, err := values.WrapMap(map[string]any{
		"VaultPublicKey": publicKey,
		"Threshold":      threshold,
	})
	require.NoError(t, err)
	raw, err := proto.Marshal(&capabilitiespb.CapabilityConfig{
		DefaultConfig: values.ProtoMap(valueMap),
	})
	require.NoError(t, err)
	return raw
}

func TestResolveVaultDON(t *testing.T) {
	t.Parallel()

	p2pA := [32]byte{1}
	p2pB := [32]byte{2}
	addrA := common.HexToAddress("0x1111111111111111111111111111111111111111")
	addrB := common.HexToAddress("0x2222222222222222222222222222222222222222")
	var signerA, signerB [32]byte
	copy(signerA[:20], addrA.Bytes())
	copy(signerB[:20], addrB.Bytes())

	reader := &mockReader{
		donIDs: []*big.Int{big.NewInt(7), big.NewInt(9)},
		dons: map[uint32]capreg.CapabilitiesRegistryDONInfo{
			7: {Id: 7, Name: "workflow-don"},
			9: {
				Id:         9,
				F:          2,
				NodeP2PIds: [][32]byte{p2pA, p2pB},
				CapabilityConfigurations: []capreg.CapabilitiesRegistryCapabilityConfiguration{
					{
						CapabilityId: vaultcommon.CapabilityID,
						Config:       vaultConfigBytes(t, "deadbeef", 2),
					},
				},
			},
		},
		nodes: []capreg.INodeInfoProviderNodeInfo{
			{P2pId: p2pA, Signer: signerA},
			{P2pId: p2pB, Signer: signerB},
		},
	}

	resolver := vaultdon.NewResolver(reader, "staging-main")
	v, err := resolver.ResolveVaultDON(context.Background())
	require.NoError(t, err)
	require.Equal(t, uint32(9), v.DON.Id)
	require.Len(t, v.Nodes, 2)

	pubKey, err := vaultdon.VaultPublicKeyHex(v)
	require.NoError(t, err)
	require.Equal(t, "deadbeef", pubKey)

	signers := capabilitiesregistry.OCRSignerAddresses(v.Nodes)
	require.Equal(t, []common.Address{addrA, addrB}, signers)
	require.Equal(t, 3, capabilitiesregistry.MinOCRSignatures(v.DON.F))

	// Cached on second call.
	v2, err := resolver.ResolveVaultDON(context.Background())
	require.NoError(t, err)
	require.Same(t, v, v2)
}

func TestResolveVaultDON_DONIDOverflow(t *testing.T) {
	t.Parallel()

	overflowID := new(big.Int).SetUint64(uint64(math.MaxUint32) + 1)
	reader := &mockReader{
		donIDs: []*big.Int{overflowID},
		dons:   map[uint32]capreg.CapabilitiesRegistryDONInfo{},
	}

	_, err := vaultdon.NewResolver(reader, "staging-main").ResolveVaultDON(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "overflows uint32")
}

func TestResolveVaultDON_NoVaultDON(t *testing.T) {
	t.Parallel()

	reader := &mockReader{
		donIDs: []*big.Int{big.NewInt(1)},
		dons: map[uint32]capreg.CapabilitiesRegistryDONInfo{
			1: {Id: 1, Name: "workflow-only"},
		},
		nodes: nil,
	}

	_, err := vaultdon.NewResolver(reader, "staging-main").ResolveVaultDON(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "no vault DON found")
}

func TestResolveVaultDON_MultipleVaultDONs(t *testing.T) {
	t.Parallel()

	cfg := vaultConfigBytes(t, "abc", 1)
	reader := &mockReader{
		donIDs: []*big.Int{big.NewInt(1), big.NewInt(2)},
		dons: map[uint32]capreg.CapabilitiesRegistryDONInfo{
			1: {
				Id: 1,
				CapabilityConfigurations: []capreg.CapabilitiesRegistryCapabilityConfiguration{
					{CapabilityId: vaultcommon.CapabilityID, Config: cfg},
				},
			},
			2: {
				Id: 2,
				CapabilityConfigurations: []capreg.CapabilitiesRegistryCapabilityConfiguration{
					{CapabilityId: vaultcommon.CapabilityID, Config: cfg},
				},
			},
		},
	}

	_, err := vaultdon.NewResolver(reader, "staging-main").ResolveVaultDON(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "multiple vault DONs")
}

func TestVaultPublicKeyHex_NilVaultDON(t *testing.T) {
	t.Parallel()
	_, err := vaultdon.VaultPublicKeyHex(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "vault DON is nil")
}

func TestParseVaultCapabilityConfig_MissingPublicKey(t *testing.T) {
	t.Parallel()

	valueMap, err := values.WrapMap(map[string]any{"Threshold": 1})
	require.NoError(t, err)
	raw, err := proto.Marshal(&capabilitiespb.CapabilityConfig{
		DefaultConfig: values.ProtoMap(valueMap),
	})
	require.NoError(t, err)

	v := &vaultdon.VaultDON{
		DON: capreg.CapabilitiesRegistryDONInfo{
			Id: 1,
			CapabilityConfigurations: []capreg.CapabilitiesRegistryCapabilityConfiguration{
				{CapabilityId: vaultcommon.CapabilityID, Config: raw},
			},
		},
	}
	_, err = vaultdon.VaultPublicKeyHex(v)
	require.Error(t, err)
	require.Contains(t, err.Error(), "VaultPublicKey")
}

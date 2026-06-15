package common

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	vaultcommon "github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	capabilitiespb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	capreg "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common/vaultdon"
)

type mockCapRegReader struct {
	donIDs []*big.Int
	dons   map[uint32]capreg.CapabilitiesRegistryDONInfo
	nodes  []capreg.INodeInfoProviderNodeInfo
}

func (m *mockCapRegReader) GetDONsInFamily(_ context.Context, _ string) ([]*big.Int, error) {
	return m.donIDs, nil
}

func (m *mockCapRegReader) GetDON(_ context.Context, donID uint32) (capreg.CapabilitiesRegistryDONInfo, error) {
	don, ok := m.dons[donID]
	if !ok {
		return capreg.CapabilitiesRegistryDONInfo{}, fmt.Errorf("DON %d not found", donID)
	}
	return don, nil
}

func (m *mockCapRegReader) GetNodes(_ context.Context) ([]capreg.INodeInfoProviderNodeInfo, error) {
	return m.nodes, nil
}

func attachMockVaultDONResolver(t *testing.T, h *Handler, vaultPublicKeyHex string) {
	t.Helper()

	valueMap, err := values.WrapMap(map[string]any{
		"VaultPublicKey": vaultPublicKeyHex,
		"Threshold":      1,
	})
	require.NoError(t, err)
	cfgBytes, err := proto.Marshal(&capabilitiespb.CapabilityConfig{
		DefaultConfig: values.ProtoMap(valueMap),
	})
	require.NoError(t, err)

	p2pID := [32]byte{1}
	reader := &mockCapRegReader{
		donIDs: []*big.Int{big.NewInt(1)},
		dons: map[uint32]capreg.CapabilitiesRegistryDONInfo{
			1: {
				Id:         1,
				F:          0,
				NodeP2PIds: [][32]byte{p2pID},
				CapabilityConfigurations: []capreg.CapabilitiesRegistryCapabilityConfiguration{
					{CapabilityId: vaultcommon.CapabilityID, Config: cfgBytes},
				},
			},
		},
		nodes: []capreg.INodeInfoProviderNodeInfo{{P2pId: p2pID}},
	}

	h.vaultDONResolver = vaultdon.NewResolver(reader, "zone-a")
	h.skipVaultValidation = false
}

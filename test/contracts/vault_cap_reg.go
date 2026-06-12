package testcontracts

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	vaultcommon "github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	capabilitiespb "github.com/smartcontractkit/chainlink-common/pkg/capabilities/pb"
	capreg "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-protos/cre/go/values"
	"github.com/smartcontractkit/chainlink-testing-framework/seth"
)

// DeployVaultCapabilitiesRegistry deploys a minimal CapabilitiesRegistry with one vault DON
// for CLI secrets integration tests.
func DeployVaultCapabilitiesRegistry(t *testing.T, sethClient *seth.Client, vaultPublicKeyHex, donFamily string) common.Address {
	t.Helper()

	opts := sethClient.NewTXOpts()
	addr, _, contract, err := capreg.DeployCapabilitiesRegistry(opts, sethClient.Client, capreg.CapabilitiesRegistryConstructorParams{
		CanAddOneNodeDONs: true,
	})
	require.NoError(t, err)

	_, err = sethClient.Decode(contract.AddCapabilities(sethClient.NewTXOpts(), []capreg.CapabilitiesRegistryCapability{
		{
			CapabilityId:            vaultcommon.CapabilityID,
			ConfigurationContract:   common.Address{},
			Metadata:                []byte{},
		},
	}))
	require.NoError(t, err)

	_, err = sethClient.Decode(contract.AddNodeOperators(sethClient.NewTXOpts(), []capreg.CapabilitiesRegistryNodeOperatorParams{
		{Admin: opts.From, Name: "test-op"},
	}))
	require.NoError(t, err)

	p2pID := [32]byte{1}
	var encKey, csaKey [32]byte
	encKey[0] = 1
	csaKey[0] = 2
	var signer [32]byte
	copy(signer[:20], opts.From.Bytes())

	_, err = sethClient.Decode(contract.AddNodes(sethClient.NewTXOpts(), []capreg.CapabilitiesRegistryNodeParams{
		{
			NodeOperatorId:      1,
			Signer:              signer,
			P2pId:               p2pID,
			EncryptionPublicKey: encKey,
			CsaKey:              csaKey,
			CapabilityIds:       []string{vaultcommon.CapabilityID},
		},
	}))
	require.NoError(t, err)

	cfgBytes := vaultCapabilityConfigBytes(t, vaultPublicKeyHex, 1)
	_, err = sethClient.Decode(contract.AddDONs(sethClient.NewTXOpts(), []capreg.CapabilitiesRegistryNewDONParams{
		{
			Name:        "vault-don",
			DonFamilies: []string{donFamily},
			CapabilityConfigurations: []capreg.CapabilitiesRegistryCapabilityConfiguration{
				{CapabilityId: vaultcommon.CapabilityID, Config: cfgBytes},
			},
			Nodes:            [][32]byte{p2pID},
			F:                0,
			IsPublic:         false,
			AcceptsWorkflows: false,
		},
	}))
	require.NoError(t, err)

	return addr
}

func vaultCapabilityConfigBytes(t *testing.T, publicKey string, threshold int) []byte {
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

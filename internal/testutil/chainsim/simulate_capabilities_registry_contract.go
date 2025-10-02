package chainsim

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/capabilities_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/cre-cli/internal/constants"
)

type SimulatedCapabilitiesRegistry struct {
	Contract   common.Address
	NodeParams []capabilities_registry_wrapper_v2.CapabilitiesRegistryNodeParams
}

func (c *SimulatedCapabilitiesRegistry) NodePublicEncryptionKeys() map[string]string {
	result := map[string]string{
		hex.EncodeToString(c.NodeParams[0].P2pId[:]): common.Bytes2Hex(c.NodeParams[0].EncryptionPublicKey[:]),
		hex.EncodeToString(c.NodeParams[1].P2pId[:]): common.Bytes2Hex(c.NodeParams[1].EncryptionPublicKey[:]),
	}

	return result
}

func (c *SimulatedCapabilitiesRegistry) NodeIds() []string {
	result := []string{}
	for _, node := range c.NodeParams {
		result = append(result, hex.EncodeToString(node.P2pId[:]))
	}
	return result
}

func DeployCapabilitiesRegistry(t *testing.T, deployerAddress string, ethClient *seth.Client, chain *SimulatedChain, logger *zerolog.Logger) SimulatedCapabilitiesRegistry {
	capRegContract, err := ethClient.DeployContractFromContractStore(
		ethClient.NewTXOpts(),
		constants.CapabilitiesRegistryContractName,
		capabilities_registry_wrapper_v2.CapabilitiesRegistryConstructorParams{
			CanAddOneNodeDONs: true,
		},
	)
	require.NoError(t, err, "Failed to deploy contract")

	logger.Info().
		Str("Address", capRegContract.Address.Hex()).
		Str("Transaction", capRegContract.Transaction.Hash().Hex()).
		Msg("CapabilitiesRegistry Contract deployed")

	capRegInstance, err := capabilities_registry_wrapper_v2.NewCapabilitiesRegistry(capRegContract.Address, ethClient.Client)
	require.NoError(t, err, "Failed to instantiate CapabilitiesRegistry contract")

	var capability capabilities_registry_wrapper_v2.CapabilitiesRegistryCapability
	capability.CapabilityId = "test-cap"
	capability.ConfigurationContract = common.HexToAddress("0x0")
	capability.Metadata = []byte{0x01, 0x02, 0x03}

	tx, err := ethClient.Decode(capRegInstance.AddCapabilities(ethClient.NewTXOpts(), []capabilities_registry_wrapper_v2.CapabilitiesRegistryCapability{capability}))
	require.NoError(t, err, "Failed to add capabilities")
	logger.Info().Str("Transaction", tx.Hash).Msg("Capabilities are set")

	capabilityId := capability.CapabilityId

	var nodeOperator capabilities_registry_wrapper_v2.CapabilitiesRegistryNodeOperator
	nodeOperator.Admin = common.HexToAddress(deployerAddress)
	nodeOperator.Name = "test-nop"

	tx, err = ethClient.Decode(capRegInstance.AddNodeOperators(ethClient.NewTXOpts(), []capabilities_registry_wrapper_v2.CapabilitiesRegistryNodeOperator{nodeOperator}))
	err = ethClient.DecodeSendErr(err)
	chain.Backend.Commit()
	require.NoError(t, err, "Failed to add node operators")
	logger.Info().Str("Transaction", tx.Hash).Msg("Node operators are set")

	var node1Params capabilities_registry_wrapper_v2.CapabilitiesRegistryNodeParams
	node1Params.EncryptionPublicKey = [32]byte(common.Hex2Bytes("1141dd1e46797ced9b0fbad49115f18507f6f6e6e3cc86e7e5ba169e58645adc"))
	node1Params.P2pId = [32]byte(common.Hex2Bytes("09ca39cd924653c72fbb0e458b629c3efebdad3e29e7cd0b5760754d919ed829"))
	node1Params.NodeOperatorId = 1
	node1Params.CapabilityIds = []string{capabilityId}
	node1Params.CsaKey = [32]byte(common.Hex2Bytes("1141dd1e46797ced9b0fbad49115f18507f6f6e6e3cc86e7e5ba169e58645adc"))
	node1Params.Signer = [32]byte(common.Hex2Bytes("d7e9f2252b09edf0802a65b60bc9956691747894cb3ab9fefd072adf742eb9f1"))

	var node2Params capabilities_registry_wrapper_v2.CapabilitiesRegistryNodeParams
	node2Params.EncryptionPublicKey = [32]byte(common.Hex2Bytes("1141dd1e46797ced9b0fbad49115f18507f6f6e6e3cc86e7e5ba169e58645adc"))
	node2Params.P2pId = [32]byte(common.Hex2Bytes("147d5cc651819b093cd2fdff9760f0f0f77b7ef7798d9e24fc6a350b7300e5d9"))
	node2Params.NodeOperatorId = 1
	node2Params.CapabilityIds = []string{capabilityId}
	node2Params.CsaKey = [32]byte(common.Hex2Bytes("1141dd1e46797ced9b0fbad49115f18507f6f6e6e3cc86e7e5ba169e58645adc"))
	node2Params.Signer = [32]byte(common.Hex2Bytes("e38c9f2252b09edf0802a65b60bc9956691747894cb3ab9fefd072adf742eb9f1"))

	nodeParams := []capabilities_registry_wrapper_v2.CapabilitiesRegistryNodeParams{node1Params, node2Params}

	tx, err = ethClient.Decode(capRegInstance.AddNodes(ethClient.NewTXOpts(), nodeParams))
	err = ethClient.DecodeSendErr(err)
	chain.Backend.Commit()
	require.NoError(t, err, "Failed to add nodes")
	logger.Info().Str("Transaction", tx.Hash).Msg("Nodes are set")

	var capConfig capabilities_registry_wrapper_v2.CapabilitiesRegistryCapabilityConfiguration
	capConfig.CapabilityId = capabilityId
	capConfig.Config = []byte{0x01, 0x02, 0x03}

	tx, err = ethClient.Decode(capRegInstance.AddDONs(ethClient.NewTXOpts(), []capabilities_registry_wrapper_v2.CapabilitiesRegistryNewDONParams{
		{
			Name:                     "test-don",
			DonFamilies:              []string{"test-family"},
			Config:                   []byte{0x01, 0x02, 0x03},
			CapabilityConfigurations: []capabilities_registry_wrapper_v2.CapabilitiesRegistryCapabilityConfiguration{capConfig},
			Nodes:                    [][32]byte{node1Params.P2pId, node2Params.P2pId},
			F:                        0,
			IsPublic:                 true,
			AcceptsWorkflows:         true,
		},
	}))
	err = ethClient.DecodeSendErr(err)
	chain.Backend.Commit()
	require.NoError(t, err, "Failed to add DON")
	logger.Info().Str("Transaction", tx.Hash).Msg("DON is set")

	// Set DON families
	tx, err = ethClient.Decode(capRegInstance.SetDONFamilies(ethClient.NewTXOpts(), 1, []string{"test-family"}, nil))
	err = ethClient.DecodeSendErr(err)
	chain.Backend.Commit()
	require.NoError(t, err, "Failed to set DON families")
	logger.Info().Str("Transaction", tx.Hash).Msg("DON families are set")

	logger.Info().
		Str("Address", capRegContract.Address.Hex()).
		Msg("CapabilitiesRegistry Contract configured and ready to use")

	return SimulatedCapabilitiesRegistry{
		Contract:   capRegContract.Address,
		NodeParams: nodeParams,
	}
}

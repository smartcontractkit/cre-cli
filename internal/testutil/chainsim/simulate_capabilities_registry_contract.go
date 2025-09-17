package chainsim

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-evm/gethwrappers/keystone/generated/capabilities_registry"
	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/dev-platform/internal/constants"
)

type SimulatedCapabilitiesRegistry struct {
	Contract   common.Address
	NodeParams []capabilities_registry.CapabilitiesRegistryNodeParams
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
	capRegContract, err := ethClient.DeployContractFromContractStore(ethClient.NewTXOpts(), constants.CapabilitiesRegistryContractName)
	require.NoError(t, err, "Failed to deploy contract")

	logger.Info().
		Str("Address", capRegContract.Address.Hex()).
		Str("Transaction", capRegContract.Transaction.Hash().Hex()).
		Msg("CapabilitiesRegistry Contract deployed")

	capRegInstance, err := capabilities_registry.NewCapabilitiesRegistry(capRegContract.Address, ethClient.Client)
	require.NoError(t, err, "Failed to instantiate CapabilitiesRegistry contract")

	var capability capabilities_registry.CapabilitiesRegistryCapability
	capability.LabelledName = "test-cap"
	capability.Version = "v1"

	tx, err := ethClient.Decode(capRegInstance.AddCapabilities(ethClient.NewTXOpts(), []capabilities_registry.CapabilitiesRegistryCapability{capability}))
	require.NoError(t, err, "Failed to add capabilities")
	logger.Info().Str("Transaction", tx.Hash).Msg("Capabilities are set")

	hashedCap, err := capRegInstance.GetHashedCapabilityId(ethClient.NewCallOpts(), capability.LabelledName, capability.Version)
	require.NoError(t, err, "Not able to fetch hashed capability")

	var nodeOperator capabilities_registry.CapabilitiesRegistryNodeOperator
	nodeOperator.Admin = common.HexToAddress(deployerAddress)
	nodeOperator.Name = "test-nop"

	tx, err = ethClient.Decode(capRegInstance.AddNodeOperators(ethClient.NewTXOpts(), []capabilities_registry.CapabilitiesRegistryNodeOperator{nodeOperator}))
	err = ethClient.DecodeSendErr(err)
	chain.Backend.Commit()
	require.NoError(t, err, "Failed to add node operators")
	logger.Info().Str("Transaction", tx.Hash).Msg("Node operators are set")

	var node1Params capabilities_registry.CapabilitiesRegistryNodeParams
	node1Params.EncryptionPublicKey = [32]byte(common.Hex2Bytes("1141dd1e46797ced9b0fbad49115f18507f6f6e6e3cc86e7e5ba169e58645adc"))
	node1Params.P2pId = [32]byte(common.Hex2Bytes("09ca39cd924653c72fbb0e458b629c3efebdad3e29e7cd0b5760754d919ed829"))
	node1Params.NodeOperatorId = 1
	node1Params.HashedCapabilityIds = append(node1Params.HashedCapabilityIds, hashedCap)
	node1Params.Signer = [32]byte(common.Hex2Bytes("d7e9f2252b09edf0802a65b60bc9956691747894cb3ab9fefd072adf742eb9f1"))

	var node2Params capabilities_registry.CapabilitiesRegistryNodeParams
	node2Params.EncryptionPublicKey = [32]byte(common.Hex2Bytes("1141dd1e46797ced9b0fbad49115f18507f6f6e6e3cc86e7e5ba169e58645adc"))
	node2Params.P2pId = [32]byte(common.Hex2Bytes("147d5cc651819b093cd2fdff9760f0f0f77b7ef7798d9e24fc6a350b7300e5d9"))
	node2Params.NodeOperatorId = 1
	node2Params.HashedCapabilityIds = append(node2Params.HashedCapabilityIds, hashedCap)
	node2Params.Signer = [32]byte(common.Hex2Bytes("e38c9f2760db006f070e9cc1bc1c2269ad033751adaa85d022fb760cbc5b5ef6"))

	nodeParams := []capabilities_registry.CapabilitiesRegistryNodeParams{node1Params, node2Params}

	tx, err = ethClient.Decode(capRegInstance.AddNodes(ethClient.NewTXOpts(), nodeParams))
	err = ethClient.DecodeSendErr(err)
	chain.Backend.Commit()
	require.NoError(t, err, "Failed to add nodes")
	logger.Info().Str("Transaction", tx.Hash).Msg("Nodes are set")

	var capConfig capabilities_registry.CapabilitiesRegistryCapabilityConfiguration
	capConfig.CapabilityId = hashedCap
	tx, err = ethClient.Decode(capRegInstance.AddDON(ethClient.NewTXOpts(),
		[][32]byte{node1Params.P2pId, node2Params.P2pId},
		[]capabilities_registry.CapabilitiesRegistryCapabilityConfiguration{capConfig},
		true,
		true,
		1,
	))
	err = ethClient.DecodeSendErr(err)
	chain.Backend.Commit()
	require.NoError(t, err, "Failed to add DON")
	logger.Info().Str("Transaction", tx.Hash).Msg("DON is set")

	logger.Info().
		Str("Address", capRegContract.Address.Hex()).
		Msg("CapabilitiesRegistry Contract configured and ready to use")

	return SimulatedCapabilitiesRegistry{
		Contract:   capRegContract.Address,
		NodeParams: nodeParams,
	}
}

package chainsim

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/dev-platform/cmd/client"
	"github.com/smartcontractkit/dev-platform/internal/constants"
)

type SimulatedWorkflowRegistry struct {
	Contract common.Address
}

func DeployWorkflowRegistry(t *testing.T, ethClient *seth.Client, chain *SimulatedChain, logger *zerolog.Logger) SimulatedWorkflowRegistry {
	deployedContract, err := ethClient.DeployContractFromContractStore(ethClient.NewTXOpts(), constants.WorkflowRegistryContractName)
	require.NoError(t, err, "Failed to deploy contract")

	workflowRegistryClient := client.NewWorkflowRegistryV2Client(logger, ethClient, deployedContract.Address.Hex(), client.Regular, &client.LedgerConfig{LedgerEnabled: false})

	err = workflowRegistryClient.UpdateAllowedSigners([]common.Address{common.HexToAddress(TestAddress)}, true)
	chain.Backend.Commit()
	require.NoError(t, err, "Failed to update authorized addresses")

	err = workflowRegistryClient.SetDonLimit(constants.DefaultProductionDonFamily, 1000, true)
	chain.Backend.Commit()
	require.NoError(t, err, "Failed to update allowed DONs")

	return SimulatedWorkflowRegistry{
		Contract: deployedContract.Address,
	}
}

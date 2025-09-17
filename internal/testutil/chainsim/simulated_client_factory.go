package chainsim

import (
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/dev-platform/cmd/client"
)

type testFactoryImpl struct {
	logger             *zerolog.Logger
	ethClient          *seth.Client
	simulatedContracts *SimulatedContracts
}

func NewSimulatedClientFactory(logger *zerolog.Logger, ethClient *seth.Client, simulatedContracts *SimulatedContracts) client.Factory {
	return &testFactoryImpl{
		logger:             logger,
		ethClient:          ethClient,
		simulatedContracts: simulatedContracts,
	}
}

func (f *testFactoryImpl) NewCapabilitiesRegistryClient() (*client.CapabilitiesRegistryClient, error) {
	return client.NewCapabilitiesRegistryClient(f.logger, f.ethClient, f.simulatedContracts.CapabilitiesRegistry.Contract), nil
}

func (f *testFactoryImpl) NewWorkflowRegistryV2Client() (*client.WorkflowRegistryV2Client, error) {
	return client.NewWorkflowRegistryV2Client(f.logger, f.ethClient, f.simulatedContracts.WorkflowRegistry.Contract.Hex(), client.Regular, &client.LedgerConfig{LedgerEnabled: false}), nil
}

func (f *testFactoryImpl) GetTxType() client.TxType {
	return client.Regular
}

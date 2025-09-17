package chainsim

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

type SimulatedEnvironment struct {
	Chain     *SimulatedChain
	EthClient *seth.Client
	Contracts *SimulatedContracts
}

type SimulatedContracts struct {
	CapabilitiesRegistry SimulatedCapabilitiesRegistry
	WorkflowRegistry     SimulatedWorkflowRegistry
}

func NewSimulatedEnvironment(t *testing.T) *SimulatedEnvironment {
	logger := testutil.NewTestLogger()

	chain := NewSimulatedChain()
	simulatedClient := NewSimulatedClient(t, chain, logger)

	capabilitiesRegistryContract := DeployCapabilitiesRegistry(t, TestAddress, simulatedClient, chain, logger)
	workflowRegistryContract := DeployWorkflowRegistry(t, simulatedClient, chain, logger)
	simulatedContracts := SimulatedContracts{
		CapabilitiesRegistry: capabilitiesRegistryContract,
		WorkflowRegistry:     workflowRegistryContract,
	}

	simulatedEnvironment := SimulatedEnvironment{
		Chain:     chain,
		EthClient: simulatedClient,
		Contracts: &simulatedContracts,
	}
	return &simulatedEnvironment
}

func (se *SimulatedEnvironment) NewRuntimeContext() *runtime.Context {
	logger := testutil.NewTestLogger()
	return se.createContextWithLogger(logger)
}

func (se *SimulatedEnvironment) NewRuntimeContextWithBufferedOutput() (*runtime.Context, *bytes.Buffer) {
	logger, buf := testutil.NewBufferedLogger()
	return se.createContextWithLogger(logger), buf
}

func (se *SimulatedEnvironment) Close() {
	se.Chain.Close()
}

func (se *SimulatedEnvironment) createContextWithLogger(logger *zerolog.Logger) *runtime.Context {
	v := viper.New()
	v.Set(settings.EthPrivateKeyEnvVar, TestPrivateKey)
	settings, err := testutil.NewTestSettings(v, logger)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to create new test settings")
	}

	simulatedFactory := NewSimulatedClientFactory(logger, se.EthClient, se.Contracts)

	environmentSet, err := environments.New()
	if err != nil {
		logger.Warn().Err(err).Msg("failed to create new environment set")
	}

	creds, err := credentials.New(logger)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to create new credentials")
	}

	return &runtime.Context{
		Logger:         logger,
		Viper:          v,
		ClientFactory:  simulatedFactory,
		Settings:       settings,
		EnvironmentSet: environmentSet,
		Credentials:    creds,
	}
}

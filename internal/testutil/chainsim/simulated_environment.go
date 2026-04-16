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
	settingspkg "github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
	"github.com/smartcontractkit/cre-cli/internal/testutil/testsettings"
)

type SimulatedEnvironment struct {
	Chain     *SimulatedChain
	EthClient *seth.Client
	Contracts *SimulatedContracts
}

type SimulatedContracts struct {
	WorkflowRegistry SimulatedWorkflowRegistry
}

func NewSimulatedEnvironment(t *testing.T) *SimulatedEnvironment {
	logger := testutil.NewTestLogger()

	chain := NewSimulatedChain()
	simulatedClient := NewSimulatedClient(t, chain, logger)

	workflowRegistryContract := DeployWorkflowRegistry(t, simulatedClient, chain, logger)
	simulatedContracts := SimulatedContracts{
		WorkflowRegistry: workflowRegistryContract,
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

// ResolvedOnChainRegistryForSimulator returns an on-chain registry whose contract address
// matches the simulator-deployed WorkflowRegistry, using chain metadata from envSet.
func (se *SimulatedEnvironment) ResolvedOnChainRegistryForSimulator(envSet *environments.EnvironmentSet) *settingspkg.OnChainRegistry {
	return settingspkg.NewOnChainRegistry(
		"",
		se.Contracts.WorkflowRegistry.Contract.Hex(),
		envSet.WorkflowRegistryChainName,
		envSet.DonFamily,
		envSet.WorkflowRegistryChainExplorerURL,
	)
}

func (se *SimulatedEnvironment) createContextWithLogger(logger *zerolog.Logger) *runtime.Context {
	v := viper.New()
	v.Set(settingspkg.EthPrivateKeyEnvVar, TestPrivateKey)
	settings, err := testsettings.NewTestSettings(v, logger)
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

	var resolved settingspkg.ResolvedRegistry
	if environmentSet != nil {
		resolved = se.ResolvedOnChainRegistryForSimulator(environmentSet)
	}

	ctx := &runtime.Context{
		Logger:           logger,
		Viper:            v,
		ClientFactory:    simulatedFactory,
		Settings:         settings,
		EnvironmentSet:   environmentSet,
		Credentials:      creds,
		ResolvedRegistry: resolved,
	}

	// Mark credentials as validated for tests to bypass validation
	if creds != nil {
		creds.IsValidated = true
	}

	return ctx
}

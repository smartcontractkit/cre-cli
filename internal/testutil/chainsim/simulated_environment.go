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
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
	"github.com/smartcontractkit/cre-cli/internal/testutil/testjwt"
	"github.com/smartcontractkit/cre-cli/internal/testutil/testsettings"
)

type SimulatedEnvironment struct {
	Chain     *SimulatedChain
	EthClient *seth.Client
	Contracts *SimulatedContracts

	tenantID           string
	donFamily          string
	deploymentRegistry string
	jwtToken           string
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

func (se *SimulatedEnvironment) WithPrivateRegistry(tenantID, donFamily string) *SimulatedEnvironment {
	se.tenantID = tenantID
	se.donFamily = donFamily
	se.deploymentRegistry = "private"
	return se
}

func (se *SimulatedEnvironment) WithJWT(orgID string) *SimulatedEnvironment {
	se.jwtToken = testjwt.CreateTestJWT(orgID)
	return se
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

func testEnvironmentSet(contractAddress string) *environments.EnvironmentSet {
	return &environments.EnvironmentSet{
		EnvName:                          environments.StagingEnv,
		WorkflowRegistryAddress:          contractAddress,
		WorkflowRegistryChainName:        "ethereum-testnet-sepolia",
		WorkflowRegistryChainExplorerURL: "https://sepolia.etherscan.io",
	}
}

func (se *SimulatedEnvironment) testTenantContext() *tenantctx.EnvironmentContext {
	tenantID := se.tenantID
	if tenantID == "" {
		tenantID = "test-tenant"
	}
	return &tenantctx.EnvironmentContext{
		TenantID:         tenantID,
		DefaultDonFamily: se.donFamily,
		Registries: []*tenantctx.Registry{
			{
				ID:    "private",
				Label: "Private (Chainlink-hosted)",
				Type:  "off-chain",
			},
		},
	}
}

func (se *SimulatedEnvironment) createContextWithLogger(logger *zerolog.Logger) *runtime.Context {
	v := viper.New()
	v.Set(settingspkg.EthPrivateKeyEnvVar, TestPrivateKey)
	settings, err := testsettings.NewTestSettings(v, logger)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to create new test settings")
	}

	simulatedFactory := NewSimulatedClientFactory(logger, se.EthClient, se.Contracts)

	environmentSet := testEnvironmentSet(se.Contracts.WorkflowRegistry.Contract.Hex())
	tenantCtx := se.testTenantContext()

	deploymentRegistry := se.deploymentRegistry
	if settings != nil {
		if se.deploymentRegistry != "" {
			settings.Workflow.UserWorkflowSettings.DeploymentRegistry = se.deploymentRegistry
		}
		if settings.Workflow.UserWorkflowSettings.DeploymentRegistry != "" {
			deploymentRegistry = settings.Workflow.UserWorkflowSettings.DeploymentRegistry
		}
	}

	var resolved settingspkg.ResolvedRegistry
	resolved, resolveErr := settingspkg.ResolveRegistry(deploymentRegistry, tenantCtx, environmentSet)
	if resolveErr != nil {
		logger.Warn().Err(resolveErr).Msg("failed to resolve deployment registry")
	}

	creds, err := credentials.New(logger)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to create new credentials")
	}

	ctx := &runtime.Context{
		Logger:           logger,
		Viper:            v,
		ClientFactory:    simulatedFactory,
		Settings:         settings,
		EnvironmentSet:   environmentSet,
		Credentials:      creds,
		ResolvedRegistry: resolved,
		TenantContext:    tenantCtx,
	}

	// Mark credentials as validated for tests to bypass validation
	if creds != nil {
		creds.IsValidated = true
		if se.jwtToken != "" {
			if creds.Tokens == nil {
				creds.Tokens = &credentials.CreLoginTokenSet{}
			}
			creds.Tokens.AccessToken = se.jwtToken
		}
	}

	return ctx
}

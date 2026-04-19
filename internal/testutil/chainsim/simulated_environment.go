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

	tenantID  string
	donFamily string
	jwtToken  string
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
	if se.tenantID != "" && se.donFamily != "" {
		resolved = settingspkg.NewOffChainRegistry("private", se.donFamily)
	} else if environmentSet != nil {
		resolved = settingspkg.NewOnChainRegistry(
			"",
			se.Contracts.WorkflowRegistry.Contract.Hex(),
			environmentSet.WorkflowRegistryChainName,
			environmentSet.DonFamily,
			environmentSet.WorkflowRegistryChainExplorerURL,
		)
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

	if se.tenantID != "" {
		ctx.TenantContext = &tenantctx.EnvironmentContext{TenantID: se.tenantID}
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

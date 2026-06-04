package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/test/multi_command_flows"
)

func setupAnvilWorkflowRegistry(t *testing.T) (*os.Process, string) {
	t.Helper()
	anvilProc, testEthURL := initTestEnv(t, "anvil-state.json")
	t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
	t.Setenv(environments.EnvVarWorkflowRegistryChainName, chainselectors.ANVIL_DEVNET.Name)
	return anvilProc, testEthURL
}

func TestWorkflow_HappyPath1_DeployPauseActivateDelete(t *testing.T) {
	isolatedEnv(t)
	anvilProc, testEthURL := setupAnvilWorkflowRegistry(t)
	defer StopAnvil(anvilProc)

	t.Setenv(credentials.CreApiKeyVar, "test-api")

	tc := NewTestConfig(t)
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3))
	require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", "", testEthURL))
	require.NoError(t, createWorkflowDirectory(tc.ProjectDirectory, "happy-path-1-workflow", "", "blank_workflow"))
	t.Cleanup(tc.Cleanup(t))

	multi_command_flows.RunHappyPath1Workflow(t, tc)
}

func TestWorkflow_HappyPath2_DeployUpdateWithConfig(t *testing.T) {
	isolatedEnv(t)
	anvilProc, testEthURL := setupAnvilWorkflowRegistry(t)
	defer StopAnvil(anvilProc)

	t.Setenv(credentials.CreApiKeyVar, "test-api")

	tc := NewTestConfig(t)
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3))
	require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", "", testEthURL))
	require.NoError(t, createWorkflowDirectory(tc.ProjectDirectory, "happy-path-2-workflow", "", "blank_workflow"))
	t.Cleanup(tc.Cleanup(t))

	multi_command_flows.RunHappyPath2Workflow(t, tc)
}

func TestWorkflow_HappyPath3a_InitDeployAutoLink(t *testing.T) {
	isolatedEnv(t)
	anvilProc, testEthURL := setupAnvilWorkflowRegistry(t)
	defer StopAnvil(anvilProc)

	t.Setenv(credentials.CreApiKeyVar, "test-api")
	t.Setenv("ETH_URL", testEthURL)

	tc := NewTestConfig(t)
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey4))
	t.Cleanup(tc.Cleanup(t))

	multi_command_flows.RunHappyPath3aWorkflow(t, tc, "happy-path-3a-project", constants.TestAddress4, testEthURL)
}

func TestWorkflow_HappyPath3b_DeployWithConfig(t *testing.T) {
	isolatedEnv(t)
	anvilProc, testEthURL := setupAnvilWorkflowRegistry(t)
	defer StopAnvil(anvilProc)

	t.Setenv(credentials.CreApiKeyVar, "test-api")

	tc := NewTestConfig(t)
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3))
	require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", "", testEthURL))
	require.NoError(t, createWorkflowDirectory(tc.ProjectDirectory, "happy-path-3b-workflow", "./config.json", "blank_workflow"))
	t.Cleanup(tc.Cleanup(t))

	multi_command_flows.RunHappyPath3bWorkflow(t, tc)
}

func TestWorkflow_PrivateRegistry_E2E(t *testing.T) {
	isolatedEnv(t)
	anvilProc, testEthURL := setupAnvilWorkflowRegistry(t)
	defer StopAnvil(anvilProc)

	t.Setenv(environments.EnvVarEnv, "STAGING")
	t.Setenv(environments.EnvVarDonFamily, "test-don")

	tc := NewTestConfig(t)
	require.NoError(t, createCliEnvFile(tc.EnvFile, ""))
	require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", "", testEthURL))
	require.NoError(t, createWorkflowDirectory(tc.ProjectDirectory, "private-registry-happy-path-workflow", "", "blank_workflow"))

	v := viper.New()
	v.SetConfigFile(filepath.Join(tc.ProjectDirectory, "blank_workflow", constants.DefaultWorkflowSettingsFileName))
	require.NoError(t, v.ReadInConfig())
	v.Set(fmt.Sprintf("%s.user-workflow.deployment-registry", SettingsTarget), "reg-test")
	require.NoError(t, v.WriteConfig())

	t.Cleanup(tc.Cleanup(t))

	multi_command_flows.RunPrivateRegistryE2E(t, tc, tc.EnvFile, filepath.Join(tc.ProjectDirectory, "blank_workflow"))
}

func TestAccount_HappyPath_LinkListUnlinkList(t *testing.T) {
	isolatedEnv(t)
	anvilProc, testEthURL := setupAnvilWorkflowRegistry(t)
	defer StopAnvil(anvilProc)

	t.Setenv(credentials.CreApiKeyVar, "test-api")

	tc := NewTestConfig(t)
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey4))
	require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", "", testEthURL))
	t.Cleanup(tc.Cleanup(t))

	multi_command_flows.RunAccountHappyPath(t, tc, testEthURL, chainselectors.ANVIL_DEVNET.Name)
}

func TestSecrets_HappyPath_CreateUpdateListDelete(t *testing.T) {
	isolatedEnv(t)
	anvilProc, testEthURL := initTestEnv(t, "anvil-state.json")
	defer StopAnvil(anvilProc)

	t.Setenv(credentials.CreApiKeyVar, "test-api")
	t.Setenv("TESTID_ENV", "testval")
	t.Setenv("TESTID_ENV_UPDATED", "testval2")

	tc := NewTestConfig(t)
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3))
	require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", "", testEthURL))
	t.Cleanup(tc.Cleanup(t))

	multi_command_flows.RunSecretsHappyPath(t, tc, chainselectors.ANVIL_DEVNET.Name)
}

func TestSecrets_ListMsig(t *testing.T) {
	isolatedEnv(t)
	anvilProc, testEthURL := initTestEnv(t, "anvil-state.json")
	defer StopAnvil(anvilProc)

	t.Setenv(credentials.CreApiKeyVar, "test-api")
	t.Setenv("TESTID_ENV", "testval")
	t.Setenv("TESTID_ENV_UPDATED", "testval2")

	tc := NewTestConfig(t)
	require.NoError(t, createCliEnvFile(tc.EnvFile, ""))
	require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", constants.TestAddress3, testEthURL))
	t.Cleanup(tc.Cleanup(t))

	multi_command_flows.RunSecretsListMsig(t, tc, chainselectors.ANVIL_DEVNET.Name)
}

func TestWorkflow_SimulationHappyPath(t *testing.T) {
	isolatedEnv(t)
	anvilProc, testEthURL := initTestEnv(t, "anvil-state-simulator.json")
	defer StopAnvil(anvilProc)

	const sepoliaForwarder = "0x15fC6ae953E024d975e77382eEeC56A9101f9F88"
	code := readDeployedBytecodeHex(t, "MockKeystoneForwarder.json")
	anvilSetCode(t, testEthURL, sepoliaForwarder, code)

	t.Setenv(credentials.CreApiKeyVar, "test-api")

	tc := NewTestConfig(t)
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3))
	require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", "", testEthURL))
	require.NoError(t, createWorkflowDirectory(tc.ProjectDirectory, "workflow-simulate", "config.json", "por_workflow"))
	t.Cleanup(tc.Cleanup(t))

	multi_command_flows.RunSimulationHappyPath(t, tc, tc.ProjectDirectory)
}

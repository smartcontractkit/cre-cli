package test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/test/multi_command_flows"
)

// Mutex to ensure all multi-command tests run sequentially to avoid context conflicts
var multiCommandTestMutex sync.Mutex

// TestMultiCommandWorkflowHappyPaths runs both happy path workflow tests sequentially
// to ensure they don't conflict with each other's context changes
func TestMultiCommandWorkflowHappyPaths(t *testing.T) {
	// Ensure sequential execution to avoid context conflicts
	multiCommandTestMutex.Lock()
	defer multiCommandTestMutex.Unlock()

	// Run Happy Path 1: Deploy -> Pause -> Activate -> Delete
	t.Run("HappyPath1_DeployPauseActivateDelete", func(t *testing.T) {
		anvilProc, testEthUrl := initTestEnv(t, "anvil-state.json")
		defer StopAnvil(anvilProc)

		// Set dummy API key for authentication
		t.Setenv(credentials.CreApiKeyVar, "test-api")

		// Setup environment variables for pre-baked registries from Anvil state dump
		t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
		t.Setenv(environments.EnvVarWorkflowRegistryChainName, chainselectors.ANVIL_DEVNET.Name)

		tc := NewTestConfig(t)

		// Use linked Address3 + its key
		require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3), "failed to create env file")
		require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", constants.TestAddress3, testEthUrl), "failed to create project.yaml")
		require.NoError(t, createWorkflowDirectory(tc.ProjectDirectory, "happy-path-1-workflow", "", "blank_workflow"), "failed to create workflow directory")
		t.Cleanup(tc.Cleanup(t))

		// Run happy path 1 workflow
		multi_command_flows.RunHappyPath1Workflow(t, tc)
	})

	// Run Happy Path 2: Deploy without autostart -> Deploy update with config
	t.Run("HappyPath2_DeployUpdateWithConfig", func(t *testing.T) {
		anvilProc, testEthUrl := initTestEnv(t, "anvil-state.json")
		defer StopAnvil(anvilProc)

		// Set dummy API key for authentication
		t.Setenv(credentials.CreApiKeyVar, "test-api")

		// Setup environment variables for pre-baked registries from Anvil state dump
		t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
		t.Setenv(environments.EnvVarWorkflowRegistryChainName, chainselectors.ANVIL_DEVNET.Name)

		tc := NewTestConfig(t)

		// Use linked Address3 + its key
		require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3), "failed to create env file")
		require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", constants.TestAddress3, testEthUrl), "failed to create project.yaml")
		require.NoError(t, createWorkflowDirectory(tc.ProjectDirectory, "happy-path-2-workflow", "", "blank_workflow"), "failed to create workflow directory")
		t.Cleanup(tc.Cleanup(t))

		// Run happy path 2 workflow
		multi_command_flows.RunHappyPath2Workflow(t, tc)
	})

	// Run Account Happy Path: Link -> List -> Unlink -> List (verify unlinked)
	t.Run("AccountHappyPath_LinkListUnlinkList", func(t *testing.T) {
		anvilProc, testEthUrl := initTestEnv(t, "anvil-state.json")
		defer StopAnvil(anvilProc)

		// Set dummy API key for authentication
		t.Setenv(credentials.CreApiKeyVar, "test-api")

		// Setup environment variables for pre-baked registries from Anvil state dump
		t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
		t.Setenv(environments.EnvVarWorkflowRegistryChainName, chainselectors.ANVIL_DEVNET.Name)

		tc := NewTestConfig(t)

		// Use test address for this test
		require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey4), "failed to create env file")
		require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", constants.TestAddress4, testEthUrl), "failed to create project.yaml")
		t.Cleanup(tc.Cleanup(t))

		// Run account happy path workflow
		multi_command_flows.RunAccountHappyPath(t, tc, testEthUrl, chainselectors.ANVIL_DEVNET.Name)
	})

	// Run Secrets Happy Path: Create -> Update -> List -> Delete
	t.Run("SecretsHappyPath_CreateUpdateListDelete", func(t *testing.T) {
		anvilProc, testEthUrl := initTestEnv(t, "anvil-state.json")
		defer StopAnvil(anvilProc)

		// Set dummy API key for authentication
		t.Setenv(credentials.CreApiKeyVar, "test-api")
		t.Setenv("TESTID_ENV", "testval")
		t.Setenv("TESTID_ENV_UPDATED", "testval2")

		tc := NewTestConfig(t)

		// Use linked Address3 + its key
		require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3), "failed to create env file")
		require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", constants.TestAddress3, testEthUrl), "failed to create project.yaml")
		t.Cleanup(tc.Cleanup(t))

		// Run secrets happy path workflow
		multi_command_flows.RunSecretsHappyPath(t, tc, chainselectors.ANVIL_DEVNET.Name)
	})

	// Run simulation
	t.Run("SimulationHappyPath", func(t *testing.T) {
		anvilProc, testEthUrl := initTestEnv(t, "anvil-state-simulator.json")
		defer StopAnvil(anvilProc)

		// Set dummy API key for authentication
		t.Setenv(credentials.CreApiKeyVar, "test-api")

		tc := NewTestConfig(t)

		// Use linked Address3 + its key
		require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3), "failed to create env file")
		require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", constants.TestAddress3, testEthUrl), "failed to create project.yaml")
		require.NoError(t, createWorkflowDirectory(tc.ProjectDirectory, "workflow-simulate", "config.json", "chainreader_workflow"), "failed to create workflow directory")
		t.Cleanup(tc.Cleanup(t))

		// Run simulation happy path workflow
		multi_command_flows.RunSimulationHappyPath(t, tc)
	})
}

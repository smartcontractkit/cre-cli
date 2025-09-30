package test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
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
		// Start fresh Anvil instance for this test
		anvilProc, testEthUrl := initTestEnv(t)
		defer StopAnvil(anvilProc)

		// Setup environment variables for pre-baked registries from Anvil state dump
		t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
		t.Setenv(environments.EnvVarWorkflowRegistryChainName, TestChainName)
		t.Setenv(environments.EnvVarCapabilitiesRegistryAddress, "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9")
		t.Setenv(environments.EnvVarCapabilitiesRegistryChainName, TestChainName)

		tc := NewTestConfig(t)

		// Use linked Address3 + its key
		require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3), "failed to create env file")
		require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", constants.TestAddress3, testEthUrl), "failed to create project.yaml")
		require.NoError(t, createWorkflowDirectory(tc.ProjectDirectory, "happy-path-1-workflow", ""), "failed to create workflow directory")
		t.Cleanup(tc.Cleanup(t))

		// Run happy path 1 workflow
		multi_command_flows.RunHappyPath1Workflow(t, tc)
	})

	// Run Happy Path 2: Deploy without autostart -> Deploy update with config
	t.Run("HappyPath2_DeployUpdateWithConfig", func(t *testing.T) {
		// Start fresh Anvil instance for this test
		anvilProc, testEthUrl := initTestEnv(t)
		defer StopAnvil(anvilProc)

		// Setup environment variables for pre-baked registries from Anvil state dump
		t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
		t.Setenv(environments.EnvVarWorkflowRegistryChainName, TestChainName)
		t.Setenv(environments.EnvVarCapabilitiesRegistryAddress, "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9")
		t.Setenv(environments.EnvVarCapabilitiesRegistryChainName, TestChainName)

		tc := NewTestConfig(t)

		// Use linked Address3 + its key
		require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3), "failed to create env file")
		require.NoError(t, createProjectSettingsFile(tc.ProjectDirectory+"project.yaml", constants.TestAddress3, testEthUrl), "failed to create project.yaml")
		require.NoError(t, createWorkflowDirectory(tc.ProjectDirectory, "happy-path-2-workflow", ""), "failed to create workflow directory")
		t.Cleanup(tc.Cleanup(t))

		// Run happy path 2 workflow
		multi_command_flows.RunHappyPath2Workflow(t, tc)
	})
}

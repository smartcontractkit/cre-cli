package settings_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

var TempWorkflowSettingsFile = filepath.Join("testdata", "workflow_storage", "workflow-with-hierarchy.yaml")
var TempProjectSettingsFile = filepath.Join("testdata", "workflow_storage", "project-with-hierarchy.yaml")

func createBlankCommand() *cobra.Command {
	return &cobra.Command{
		Use: "workflow",
	}
}

func TestSettingsHierarchy(t *testing.T) {
	// Get absolute paths for template files
	workflowTemplatePath, err := filepath.Abs(TempWorkflowSettingsFile)
	require.NoError(t, err, "Error when resolving workflow template path")

	projectTemplatePath, err := filepath.Abs(TempProjectSettingsFile)
	require.NoError(t, err, "Error when resolving project template path")

	// Create temporary directory and change to it
	tempDir := t.TempDir()
	restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err, "Error changing working directory")
	defer restoreWorkingDirectory()

	// Copy test files to the temporary directory
	workflowFilePath := filepath.Join(tempDir, constants.DefaultWorkflowSettingsFileName)
	require.NoError(t, copyFile(workflowTemplatePath, workflowFilePath), "Error copying workflow file")

	projectFilePath := filepath.Join(tempDir, constants.DefaultProjectSettingsFileName)
	require.NoError(t, copyFile(projectTemplatePath, projectFilePath), "Error copying project file")

	// Set up viper and load settings
	blankCmd := createBlankCommand()
	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "production-testnet")

	err = settings.LoadSettingsIntoViper(v, blankCmd)
	require.NoError(t, err, "Error when loading settings")

	hierarchyVal := v.GetString("production-testnet.hierarchy-test")
	require.Equal(t, "Workflow", hierarchyVal)

	testVal := v.GetString("production-testnet.test-key")
	require.Equal(t, "workflowValue", testVal)
}

// TODO: happy path unit test, write more edge case tests
func TestLoadingSettingsForValidFile(t *testing.T) {
	// Get absolute paths for template files
	workflowTemplatePath, err := filepath.Abs(TempWorkflowSettingsFile)
	require.NoError(t, err, "Error when resolving workflow template path")

	projectTemplatePath, err := filepath.Abs(TempProjectSettingsFile)
	require.NoError(t, err, "Error when resolving project template path")

	// Create temporary directory and change to it
	tempDir := t.TempDir()
	restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(tempDir)
	require.NoError(t, err, "Error changing working directory")
	defer restoreWorkingDirectory()

	// Copy test files to the temporary directory
	workflowFilePath := filepath.Join(tempDir, constants.DefaultWorkflowSettingsFileName)
	require.NoError(t, copyFile(workflowTemplatePath, workflowFilePath), "Error copying workflow file")

	projectFilePath := filepath.Join(tempDir, constants.DefaultProjectSettingsFileName)
	require.NoError(t, copyFile(projectTemplatePath, projectFilePath), "Error copying project file")

	// Set up viper and load settings
	blankCmd := createBlankCommand()
	v := viper.New()
	v.Set(settings.CreTargetEnvVar, "production-testnet")

	err = settings.LoadSettingsIntoViper(v, blankCmd)
	require.NoError(t, err, "Error when loading settings")

	rpcUrl, err := settings.GetRpcUrlSettings(v, "ethereum-testnet-sepolia-arbitrum-1")
	require.NoError(t, err, "RPC URL not found")
	require.Equal(t, "https://somethingElse.rpc.org", rpcUrl)

	rpcUrl, err = settings.GetRpcUrlSettings(v, "ethereum-testnet-sepolia")
	require.NoError(t, err, "RPC URL not found")
	require.Equal(t, "https://something.rpc.org", rpcUrl)
}

func TestLoadEnvFromParent(t *testing.T) {
	tempDir := t.TempDir()

	parentDir := filepath.Join(tempDir, "envparent")
	err := os.Mkdir(parentDir, 0755)
	require.NoError(t, err, "unable to create parent directory")

	childDir, err := os.MkdirTemp(parentDir, "envchild")
	require.NoError(t, err, "unable to create temporary child directory")

	envFilePath := filepath.Join(parentDir, constants.DefaultEnvFileName)
	envContent := "TEST_VAR=from_parent\n"
	err = os.WriteFile(envFilePath, []byte(envContent), 0600)
	require.NoError(t, err, "unable to write .env file")

	os.Unsetenv("TEST_VAR")
	os.Unsetenv("CRE_TARGET")

	restoreWorkingDirectory, err := testutil.ChangeWorkingDirectory(childDir)
	require.NoError(t, err, "unable to change working directory to child directory")
	defer restoreWorkingDirectory()

	err = settings.LoadEnv(".env")
	require.NoError(t, err, "LoadEnv() failed to load the .env file from a parent directory")

	require.Equal(t, "from_parent", os.Getenv("TEST_VAR"), "TEST_VAR should have been loaded from the .env file")
	require.Empty(t, os.Getenv("TARGET"), "TARGET should not be set in the configuration")
}

package settings_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/settings"
	"github.com/smartcontractkit/dev-platform/internal/testutil"
	"github.com/smartcontractkit/dev-platform/internal/transformation"
)

const ValidProjectSettingsFile = `
production-testnet:
  hierarchy-test: Project
  test-key: projectValue
  dev-platform:
    don-id: 1
  user-workflow:
    workflow-owner-address: ""
    workflow-name: ""
  logging:
    seth-config-path: seth.toml
  contracts:
    registries:
      - name: WorkflowRegistry
        address: "0x0E974d80e38DC52d90afBaa3745FDB71C723613b"
        chain-selector: 3478487238524512106
      - name: CapabilitiesRegistry
        address: "0x531eF7673958E227002fBe8FF6A97DC17111f815"
        chain-selector: 16015286601757825753
    data-feeds:
      - name: DataFeedsCache
        address: "0x4B835DaeEE75127A4637046c7DA54c86c148Bfd1"
        chain-selector: 16015286601757825753
      - name: DataFeedsCache
        address: "0xd00ce6061BfC404801FA7a965A0c4dA2fDbA067f"
        chain-selector: 3478487238524512106
      - name: BundleAggregatorProxy
        address: "0xD1ADC316653733e075c43f5daF7e7f9867df6B02"
        chain-selector: 3478487238524512106
  rpcs:
    - chain-selector: 3478487238524512106
      url: https://somethingElse.rpc.org
    - chain-selector: 16015286601757825753
      url: https://something.rpc.org

`

var TempWorkflowSettingsFile = filepath.Join("testdata", "workflow_storage", "workflow-with-hierarchy.yaml")
var TempProjectSettingsFile = filepath.Join("testdata", "workflow_storage", "project-with-hierarchy.yaml")

func TestSettingsHierarchy(t *testing.T) {
	//Create project settings file
	err := os.WriteFile(constants.DefaultProjectSettingsFileName, []byte(ValidProjectSettingsFile), 0600)
	require.NoError(t, err, "Not able to write project settings file")

	absPathWorkflow, err := transformation.ResolvePath(TempWorkflowSettingsFile)
	require.NoError(t, err, "Error when resolving settings path")

	v := viper.New()
	v.Set(settings.Flags.CliSettingsFile.Name, absPathWorkflow)
	v.Set(settings.CreTargetEnvVar, "production-testnet")

	v.SetConfigFile(constants.DefaultProjectSettingsFileName)
	err = settings.LoadSettingsIntoViper(v)
	require.NoError(t, err, "Error when loading settings")

	hierarchyVal := v.GetString("production-testnet.hierarchy-test")
	require.Equal(t, "Workflow", hierarchyVal)

	testVal := v.GetString("production-testnet.test-key")
	require.Equal(t, "workflowValue", testVal)

	err = os.Remove(constants.DefaultProjectSettingsFileName)
	require.NoError(t, err, "Not able to remove settings file")

}

// TODO: happy path unit test, write more edge case tests
func TestLoadingSettingsForValidFile(t *testing.T) {
	//Create project settings file
	err := os.WriteFile(constants.DefaultProjectSettingsFileName, []byte(ValidProjectSettingsFile), 0600)
	require.NoError(t, err, "Not able to write project settings file")

	absPath, err := transformation.ResolvePath(TempWorkflowSettingsFile)
	require.NoError(t, err, "Error when resolving settings path")

	v := viper.New()
	v.Set(settings.Flags.CliSettingsFile.Name, absPath)
	v.Set(settings.CreTargetEnvVar, "production-testnet")

	v.SetConfigFile(constants.DefaultProjectSettingsFileName)
	err = settings.LoadSettingsIntoViper(v)
	require.NoError(t, err, "Error when loading settings")

	rpcUrl, err := settings.GetRpcUrlSettings(v, uint64(3478487238524512106))
	require.NoError(t, err, "RPC URL not found")
	require.Equal(t, "https://somethingElse.rpc.org", rpcUrl)

	rpcUrl, err = settings.GetRpcUrlSettings(v, uint64(16015286601757825753))
	require.NoError(t, err, "RPC URL not found")
	require.Equal(t, "https://something.rpc.org", rpcUrl)

	err = os.Remove(constants.DefaultProjectSettingsFileName)
	require.NoError(t, err, "Not able to remove settings file")

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

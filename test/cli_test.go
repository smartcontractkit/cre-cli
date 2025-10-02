package test

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	chainselectors "github.com/smartcontractkit/chain-selectors"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func createProjectSettingsFile(projectSettingPath string, workflowOwner string, testEthURL string) error {
	v := viper.New()

	v.Set(fmt.Sprintf("%s.%s", SettingsTarget, settings.DONFamilySettingName), constants.DefaultStagingDonFamily)

	// account fields
	if workflowOwner != "" {
		v.Set(fmt.Sprintf("%s.account.workflow-owner-address", SettingsTarget), workflowOwner)
	}

	// cre-cli fields
	v.Set(fmt.Sprintf("%s.cre-cli.don-family", SettingsTarget), constants.DefaultStagingDonFamily)

	// rpcs
	v.Set(fmt.Sprintf("%s.%s", SettingsTarget, settings.RpcsSettingName), []settings.RpcEndpoint{
		{
			ChainName: chainselectors.ANVIL_DEVNET.Name,
			Url:       testEthURL,
		},
		{
			Url:       testEthURL,
			ChainName: chainselectors.ETHEREUM_TESTNET_SEPOLIA.Name,
		},
	})

	// write YAML
	v.SetConfigType("yaml")
	if err := v.WriteConfigAs(projectSettingPath); err != nil {
		return fmt.Errorf("error writing project.yaml: %w", err)
	}

	L.Debug().
		Str("ProjectSettingsFile", projectSettingPath).
		Interface("Config", v.AllSettings()).
		Msg("Project settings file created")

	return nil
}

func createCliEnvFile(envPath string, ethPrivateKey string) error {
	file, err := os.OpenFile(envPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	_, err = writer.WriteString("\n")
	if err != nil {
		return err
	}
	_, err = writer.WriteString(fmt.Sprintf("%s=%s", settings.EthPrivateKeyEnvVar, ethPrivateKey))
	if err != nil {
		return err
	}

	_, err = writer.WriteString("\n")
	if err != nil {
		return err
	}

	_, err = writer.WriteString(fmt.Sprintf("%s=%s", settings.CreTargetEnvVar, SettingsTarget))
	if err != nil {
		return err
	}

	_, err = writer.WriteString("\n")
	if err != nil {
		return err
	}
	writer.Flush()

	return nil
}

// createWorkflowDirectory creates the workflow directory with test files and workflow.yaml
func createWorkflowDirectory(
	projectDirectory string,
	workflowName string,
	workflowConfigPath string,
	workflowDirectoryName string,
) error {
	trimmedName := strings.TrimSpace(workflowName)
	if len(trimmedName) < 1 {
		return fmt.Errorf("workflow name %q is too short, minimum length is 1 character", trimmedName)
	}

	// Get the source workflow directory
	_, thisFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(thisFile)
	sourceWorkflowDir := filepath.Join(testDir, "test_project", workflowDirectoryName)

	// Create workflow directory in project
	workflowDir := filepath.Join(projectDirectory, workflowDirectoryName)
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		return fmt.Errorf("failed to create workflow directory: %w", err)
	}

	// Copy workflow files
	items := []string{"main.go", "config.json", "go.mod", "go.sum", "contracts"}
	for _, item := range items {
		src := filepath.Join(sourceWorkflowDir, item)
		dst := filepath.Join(workflowDir, item)

		// Skip if the item doesn't exist
		if _, err := os.Stat(src); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("failed to stat %s: %w", item, err)
		}

		if err := copyPath(src, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", item, err)
		}
	}

	// Create workflow.yaml file using viper
	workflowSettingsPath := filepath.Join(workflowDir, constants.DefaultWorkflowSettingsFileName)

	v := viper.New()

	// user-workflow fields
	v.Set(fmt.Sprintf("%s.user-workflow.workflow-name", SettingsTarget), trimmedName)

	// workflow-artifacts - initially create without config-path for first deployment
	workflowArtifacts := map[string]string{
		"workflow-path": "./main.go",
	}
	// Only add config-path if explicitly provided
	if workflowConfigPath != "" {
		workflowArtifacts["config-path"] = workflowConfigPath
	}
	v.Set(fmt.Sprintf("%s.workflow-artifacts", SettingsTarget), workflowArtifacts)

	// write YAML
	v.SetConfigType("yaml")
	if err := v.WriteConfigAs(workflowSettingsPath); err != nil {
		return fmt.Errorf("error writing workflow.yaml: %w", err)
	}

	L.Debug().
		Str("WorkflowSettingsFile", workflowSettingsPath).
		Interface("Config", v.AllSettings()).
		Msg("Workflow settings file created")

	return nil
}

func initTestEnv(t *testing.T, stateFileName string) (*os.Process, string) {
	InitLogging()
	anvilProc, anvilPort, err := StartAnvil(LOAD_ANVIL_STATE, stateFileName)
	require.NoError(t, err, "Failed to start Anvil")
	ethUrl := "http://localhost:" + strconv.Itoa(anvilPort)
	return anvilProc, ethUrl
}

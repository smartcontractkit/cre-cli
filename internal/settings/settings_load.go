package settings

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/transformation"
)

// Config names (YAML field paths)
const (
	DONFamilySettingName      = "dev-platform.don-family"
	WorkflowOwnerSettingName  = "user-workflow.workflow-owner-address"
	WorkflowNameSettingName   = "user-workflow.workflow-name"
	SethConfigPathSettingName = "logging.seth-config-path"
	RegistriesSettingName     = "contracts.registries"
	KeystoneSettingName       = "contracts.keystone"
	RpcsSettingName           = "rpcs"
)

type Flag struct {
	Name  string
	Short string
}

type flagNames struct {
	Owner                Flag
	CliSettingsFile      Flag
	CliEnvFile           Flag
	Verbose              Flag
	Target               Flag
	OverridePreviousRoot Flag
	Description          Flag
	RawTxFlag            Flag
	Ledger               Flag
	LedgerDerivationPath Flag
}

var Flags = flagNames{
	Owner:                Flag{"owner", "o"},
	CliSettingsFile:      Flag{"workflow-settings-file", "S"},
	CliEnvFile:           Flag{"env", "e"},
	Verbose:              Flag{"verbose", "v"},
	Target:               Flag{"target", "T"},
	OverridePreviousRoot: Flag{"override-previous-root", "O"},
	RawTxFlag:            Flag{"unsigned", ""},
	Ledger:               Flag{"ledger", ""},
	LedgerDerivationPath: Flag{"ledger-derivation-path", ""},
}

func AddTxnTypeFlags(cmd *cobra.Command) {
	AddRawTxFlag(cmd)
	cmd.Flags().Bool(Flags.Ledger.Name, false, "Sign the workflow with a Ledger device [EXPERIMENTAL]")
	cmd.Flags().String(Flags.LedgerDerivationPath.Name, "m/44'/60'/0'/0/0", "Derivation path for the Ledger device")
}

func AddRawTxFlag(cmd *cobra.Command) {
	cmd.Flags().Bool(Flags.RawTxFlag.Name, false, "If set, the command will return the raw transaction instead of sending it to the network [EXPERIMENTAL]")
}

func FindProjectSettingsPath(startDir string) (string, bool, error) {
	var err error

	if startDir == "" {
		return "", false, fmt.Errorf("starting directory cannot be empty")
	}

	cwd := startDir

	for {
		filePath := filepath.Join(cwd, constants.DefaultProjectSettingsFileName)
		if _, err = os.Stat(filePath); err == nil {
			return filePath, true, nil // File exists, return the path and true
		} else if !os.IsNotExist(err) {
			return "", false, fmt.Errorf("error checking project settings: %w", err)
		}

		parentDir := filepath.Dir(cwd)
		if parentDir == cwd {
			break // Stop if we can't go up further
		}
		cwd = parentDir
	}

	return "", false, nil
}

func mergeConfigToViper(v *viper.Viper, filePath string) error {
	v.SetConfigFile(filePath)
	err := v.MergeInConfig()
	if err != nil {
		return fmt.Errorf("error loading config file %s: %w", filePath, err)
	}
	return nil
}

// Loads the configuration file (if found) and sets the configuration values via Viper
func LoadSettingsIntoViper(v *viper.Viper) error {
	workflowSettingsPath, err := transformation.ResolvePath(v.GetString(Flags.CliSettingsFile.Name))
	if err != nil {
		return fmt.Errorf("failed to find workflow settings (%s): %w", constants.DefaultWorkflowSettingsFileName, err)
	}

	var projectSettingsPath string

	if v.IsSet("projectSettingsPath") {
		projectSettingsPath, err = transformation.ResolvePath(v.GetString("projectSettingsPath"))
		if err != nil {
			return fmt.Errorf("cannot resolve path to project settings (%s): %w", constants.DefaultProjectSettingsFileName, err)
		}
	} else {
		projectSettingsPath, err = getProjectSettingsPath()
		if err != nil {
			return fmt.Errorf("failed to find project settings (%s): %w", constants.DefaultProjectSettingsFileName, err)
		}
	}

	v.SetConfigType("yaml")
	if err := mergeConfigToViper(v, projectSettingsPath); err != nil {
		return fmt.Errorf("failed to load project settings: %w", err)
	}

	// Step 2: Load workflow settings next (overwrites values from project settings)
	if err := mergeConfigToViper(v, workflowSettingsPath); err != nil {
		return fmt.Errorf("failed to load workflow settings: %w", err)
	}

	return nil
}

func getProjectSettingsPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	path, isFound, err := FindProjectSettingsPath(cwd)
	if err != nil {
		return "", err
	}

	if !isFound {
		return "", fmt.Errorf("failed to find project settings")
	}

	return path, nil
}

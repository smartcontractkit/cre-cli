package settings

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/context"
)

// Config names (YAML field paths)
const (
	WorkflowOwnerSettingName  = "account.workflow-owner-address"
	WorkflowNameSettingName   = "user-workflow.workflow-name"
	WorkflowPathSettingName   = "workflow-artifacts.workflow-path"
	ConfigPathSettingName     = "workflow-artifacts.config-path"
	SecretsPathSettingName    = "workflow-artifacts.secrets-path"
	OverrideFilePathSettingName = "workflow-artifacts.override-file-path"
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
	ProjectRoot          Flag
	CliEnvFile           Flag
	Verbose              Flag
	Target               Flag
	OverridePreviousRoot Flag
	Description          Flag
	RawTxFlag            Flag
	Changeset            Flag
	Ledger               Flag
	LedgerDerivationPath Flag
	NonInteractive       Flag
	SkipConfirmation     Flag
	ChangesetFile        Flag
}

var Flags = flagNames{
	Owner:                Flag{"owner", "o"},
	ProjectRoot:          Flag{"project-root", "R"},
	CliEnvFile:           Flag{"env", "e"},
	Verbose:              Flag{"verbose", "v"},
	Target:               Flag{"target", "T"},
	OverridePreviousRoot: Flag{"override-previous-root", "O"},
	RawTxFlag:            Flag{"unsigned", ""},
	Changeset:            Flag{"changeset", ""},
	Ledger:               Flag{"ledger", ""},
	LedgerDerivationPath: Flag{"ledger-derivation-path", ""},
	NonInteractive:       Flag{"non-interactive", ""},
	SkipConfirmation:     Flag{"yes", "y"},
	ChangesetFile:        Flag{"changeset-file", ""},
}

func AddTxnTypeFlags(cmd *cobra.Command) {
	cmd.Flags().Bool(Flags.RawTxFlag.Name, false, "If set, the command will either return the raw transaction instead of sending it to the network or execute the second step of secrets operations using a previously generated raw transaction")
	cmd.Flags().Bool(Flags.Changeset.Name, false, "If set, the command will output a changeset YAML for use with CLD instead of sending the transaction to the network")
	cmd.Flags().String(Flags.ChangesetFile.Name, "", "If set, the command will append the generated changeset to the specified file")
	_ = cmd.LocalFlags().MarkHidden(Flags.Changeset.Name)     // hide changeset flag as this is not a public feature
	_ = cmd.LocalFlags().MarkHidden(Flags.ChangesetFile.Name) // hide changeset flag as this is not a public feature
	//	cmd.Flags().Bool(Flags.Ledger.Name, false, "Sign the workflow with a Ledger device [EXPERIMENTAL]")
	//	cmd.Flags().String(Flags.LedgerDerivationPath.Name, "m/44'/60'/0'/0/0", "Derivation path for the Ledger device")
}

func AddSkipConfirmation(cmd *cobra.Command) {
	cmd.Flags().Bool(Flags.SkipConfirmation.Name, false, "If set, the command will skip the confirmation prompt and proceed with the operation even if it is potentially destructive")
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
func LoadSettingsIntoViper(v *viper.Viper, cmd *cobra.Command) error {
	projectSettingsPath, err := getProjectSettingsPath()
	if err != nil {
		return fmt.Errorf("failed to find project settings (%s): %w", constants.DefaultProjectSettingsFileName, err)
	}

	v.SetConfigType("yaml")
	if err := mergeConfigToViper(v, projectSettingsPath); err != nil {
		return fmt.Errorf("failed to load project settings: %w", err)
	}

	if context.IsWorkflowCommand(cmd) {
		// Step 2: Load workflow settings next (overwrites values from project settings)
		if err := mergeConfigToViper(v, constants.DefaultWorkflowSettingsFileName); err != nil {
			return fmt.Errorf("failed to load workflow settings: %w", err)
		}
	}

	return nil
}

func getProjectSettingsPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	path, isFound, err := context.FindProjectSettingsPath(cwd)
	if err != nil {
		return "", err
	}

	if !isFound {
		return "", fmt.Errorf("failed to find project settings")
	}

	return path, nil
}

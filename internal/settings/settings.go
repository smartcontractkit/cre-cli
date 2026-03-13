package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// sensitive information (not in configuration file)
const (
	EthPrivateKeyEnvVar = "CRE_ETH_PRIVATE_KEY"
	CreTargetEnvVar     = "CRE_TARGET"
)

// State tracked by LoadEnv so downstream code (e.g. build warnings) can
// inspect what happened without re-discovering or re-parsing the file.
var (
	loadedEnvFilePath string
	loadedEnvVars     map[string]string
)

// LoadedEnvFilePath returns the path that was successfully loaded, or "".
func LoadedEnvFilePath() string { return loadedEnvFilePath }

// LoadedEnvVars returns the key-value pairs parsed from the loaded .env file.
// Returns nil if no file was loaded.
func LoadedEnvVars() map[string]string { return loadedEnvVars }

// Settings holds user, project, and workflow configurations.
type Settings struct {
	Workflow        WorkflowSettings
	User            UserSettings
	StorageSettings WorkflowStorageSettings
	CLDSettings     CLDSettings
}

// UserSettings stores user-specific configurations.
type UserSettings struct {
	TargetName    string
	EthPrivateKey string
	EthUrl        string
}

// New initializes and loads settings from YAML config files and the environment.
// Environment loading (.env + BindEnv) is handled earlier in PersistentPreRunE
// so that all commands see the variables consistently.
func New(logger *zerolog.Logger, v *viper.Viper, cmd *cobra.Command, registryChainName string) (*Settings, error) {
	target, err := GetTarget(v)
	if err != nil {
		return nil, err
	}

	if target == "" {
		if v.GetBool(Flags.NonInteractive.Name) {
			target, err = autoSelectTarget(logger)
		} else {
			target, err = promptForTarget(logger)
		}
		if err != nil {
			return nil, err
		}
		// Store the selected target so subsequent GetTarget() calls find it
		v.Set(Flags.Target.Name, target)
	}

	logger.Debug().Msgf("Target:  %s", target)

	err = LoadSettingsIntoViper(v, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	workflowSettings, err := loadWorkflowSettings(logger, v, cmd, registryChainName)
	if err != nil {
		return nil, err
	}
	storageSettings := LoadWorkflowStorageSettings(logger, v)

	cldSettings, err := loadCLDSettings(logger, v, cmd, registryChainName)
	if err != nil {
		return nil, err
	}

	rawPrivKey := v.GetString(EthPrivateKeyEnvVar)
	normPrivKey := NormalizeHexKey(rawPrivKey)

	return &Settings{
		User: UserSettings{
			EthPrivateKey: normPrivKey,
			TargetName:    target,
		},
		Workflow:        workflowSettings,
		StorageSettings: storageSettings,
		CLDSettings:     cldSettings,
	}, nil
}

// LoadEnv loads environment variables from envPath into the process
// environment, binds sensitive variables into Viper, and logs outcomes.
// If envPath is empty no file is loaded and a debug message is emitted.
// Errors are logged but do not halt execution — the CLI continues so
// that commands which don't need the env file can still run.
func LoadEnv(logger *zerolog.Logger, v *viper.Viper, envPath string) {
	loadedEnvFilePath = ""
	loadedEnvVars = nil

	if envPath == "" {
		logger.Debug().Msg(
			"No environment file specified and .env was not found in the current or parent directories. " +
				"CLI tool will read individual environment variables (they MUST be exported).")
		return
	}

	if err := godotenv.Overload(envPath); err != nil {
		logger.Error().Str("path", envPath).Err(err).Msg(
			"Not able to load configuration from .env file. " +
				"CLI tool will read and verify individual environment variables (they MUST be exported). " +
				"If the .env file is present, please check that it follows the correct format: https://dotenvx.com/docs/env-file")
		return
	}

	loadedEnvFilePath = envPath
	loadedEnvVars, _ = godotenv.Read(envPath)

	if err := bindEnv(v); err != nil {
		logger.Error().Err(err).Msg(
			"Not able to bind environment variables that represent sensitive data. " +
				"They are required for the CLI tool to function properly, without them some commands may not work. " +
				"Please export them manually or set via .env file (check example.env for more information).")
	}
}

func bindEnv(v *viper.Viper) error {
	envVars := []string{
		EthPrivateKeyEnvVar,
		CreTargetEnvVar,
	}

	for _, variable := range envVars {
		if err := v.BindEnv(variable); err != nil {
			return fmt.Errorf("failed to bind environment variable: %s", variable)
		}
	}

	v.AutomaticEnv()
	return nil
}

// FindEnvFile walks up from startDir looking for a file named fileName.
func FindEnvFile(startDir, fileName string) (string, error) {
	dir := startDir

	for {
		filePath := filepath.Join(dir, fileName)

		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			return filePath, nil
		}

		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			break // Reached the root directory.
		}
		dir = parentDir
	}
	return "", fmt.Errorf("file %s not found in any parent directory starting from %s", fileName, startDir)
}

func NormalizeHexKey(k string) string {
	k = strings.TrimSpace(k)
	if len(k) >= 2 && (k[0:2] == "0x" || k[0:2] == "0X") {
		return k[2:]
	}
	return k
}

// autoSelectTarget discovers available targets and auto-selects when possible (non-interactive mode).
func autoSelectTarget(logger *zerolog.Logger) (string, error) {
	targets, err := GetAvailableTargets()
	if err != nil {
		return "", fmt.Errorf("target not set and unable to discover targets: %w\nSpecify --%s or set %s env var",
			err, Flags.Target.Name, CreTargetEnvVar)
	}

	if len(targets) == 0 {
		return "", fmt.Errorf("no targets found in project.yaml; specify --%s or set %s env var",
			Flags.Target.Name, CreTargetEnvVar)
	}

	if len(targets) == 1 {
		logger.Debug().Msgf("Auto-selecting target: %s", targets[0])
		return targets[0], nil
	}

	return "", fmt.Errorf("multiple targets found in project.yaml and --non-interactive is set; specify --%s or set %s env var",
		Flags.Target.Name, CreTargetEnvVar)
}

// promptForTarget discovers available targets from project.yaml and prompts the user to select one.
func promptForTarget(logger *zerolog.Logger) (string, error) {
	targets, err := GetAvailableTargets()
	if err != nil {
		return "", fmt.Errorf("target not set and unable to discover targets: %w\nSpecify --%s or set %s env var",
			err, Flags.Target.Name, CreTargetEnvVar)
	}

	if len(targets) == 0 {
		return "", fmt.Errorf("no targets found in project.yaml; specify --%s or set %s env var",
			Flags.Target.Name, CreTargetEnvVar)
	}

	if len(targets) == 1 {
		logger.Debug().Msgf("Auto-selecting target: %s", targets[0])
		return targets[0], nil
	}

	var selected string
	options := make([]huh.Option[string], len(targets))
	for i, t := range targets {
		options[i] = huh.NewOption(t, t)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a target").
				Description("No --target flag or CRE_TARGET env var set.").
				Options(options...).
				Value(&selected),
		),
	).WithTheme(ui.ChainlinkTheme())

	if err := form.Run(); err != nil {
		return "", fmt.Errorf("target selection cancelled: %w", err)
	}

	return selected, nil
}

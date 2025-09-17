package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/internal/constants"
)

// sensitive information (not in configuration file)
const (
	EthPrivateKeyEnvVar = "CRE_ETH_PRIVATE_KEY"
	CreTargetEnvVar     = "CRE_TARGET"
)

const loadEnvErrorMessage = "Not able to load configuration from .env file, skipping this optional step.\n" +
	"CLI tool will read and verify individual environment variables (they MUST be exported).\n" +
	"If you want to use .env file, please check that you are fetching the .env file from the correct location.\n" +
	"Note that if .env location is not provided via CLI flag, default is .env file located in the current working directory where the CLI tool runs.\n" +
	"If .env file doesn't exist, it has to be created first (check example.env for more information).\n" +
	"If the .env file is present, please check that it follows the correct format: https://dotenvx.com/docs/env-file"

const bindEnvErrorMessage = "Not able to bind environment variables that represent sensitive data.\n" +
	"They are required for the CLI tool to function properly, without them some commands may not work.\n" +
	"Please export them manually or set via .env file (check example.env for more information)."

// Settings holds user, project, and workflow configurations.
type Settings struct {
	Workflow        WorkflowSettings
	User            UserSettings
	StorageSettings WorkflowStorageSettings
}

// UserSettings stores user-specific configurations.
type UserSettings struct {
	TargetName    string
	EthPrivateKey string
	EthUrl        string
}

// New initializes and loads settings from the `.env` file or system environment.
func New(logger *zerolog.Logger, v *viper.Viper) (*Settings, error) {
	// Retrieve the flag value (user-provided or default)
	envPath := v.GetString(Flags.CliEnvFile.Name)

	// try to load the .env file (fetch sensitive info)
	if err := LoadEnv(envPath); err != nil {
		// .env file is optional, so we log it as a debug message
		logger.Debug().Msg(loadEnvErrorMessage)
	}

	// try to bind sensitive environment variables (loaded from .env file or manually exported to
	// shell environment)
	if err := BindEnv(v); err != nil {
		// not necessarily an issue, more like a warning
		logger.Debug().Err(err).Msg(bindEnvErrorMessage)
	}

	target, err := GetTarget(v)
	if err != nil {
		return nil, err
	}

	logger.Debug().Msgf("Target:  %s", target)

	err = LoadSettingsIntoViper(v)
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	workflowSettings, err := loadWorkflowSettings(logger, v)
	if err != nil {
		return nil, err
	}
	storageSettings := LoadWorkflowStorageSettings(logger, v)

	rawPrivKey := v.GetString(EthPrivateKeyEnvVar)
	normPrivKey := NormalizeHexKey(rawPrivKey)

	return &Settings{
		User: UserSettings{
			EthPrivateKey: normPrivKey,
			TargetName:    target,
		},
		Workflow:        workflowSettings,
		StorageSettings: storageSettings,
	}, nil
}

func BindEnv(v *viper.Viper) error {
	envVars := []string{
		EthPrivateKeyEnvVar,
		CreTargetEnvVar,
	}

	for _, variable := range envVars {
		if err := v.BindEnv(variable); err != nil {
			return fmt.Errorf("failed to bind environment variable: %s", variable)
		}
	}

	v.AutomaticEnv() // Ensure variables are picked up
	return nil
}

func LoadEnv(envPath string) error {
	if envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			if err := godotenv.Load(envPath); err != nil {
				return fmt.Errorf("error loading file from %s: %w", envPath, err)
			}
			return nil
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting working directory: %w", err)
	}

	foundEnvPath, err := findEnvFile(cwd, constants.DefaultEnvFileName)
	if err != nil {
		return fmt.Errorf("error loading environment: %w", err)
	}

	if err := godotenv.Load(foundEnvPath); err != nil {
		return fmt.Errorf("error loading file from %s: %w", foundEnvPath, err)
	}
	return nil
}

func findEnvFile(startDir, fileName string) (string, error) {
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

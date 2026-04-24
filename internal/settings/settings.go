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

	corekeys "github.com/smartcontractkit/chainlink-common/keystore/corekeys"

	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const CreTargetEnvVar = "CRE_TARGET"

// ChainType describes a chain family and the per-family settings the CLI
// loads from the environment. Add a family by appending to AllChainTypes.
type ChainType struct {
	Name          string
	PrivateKeyEnv string
}

var (
	EVM = ChainType{
		Name:          string(corekeys.EVM),
		PrivateKeyEnv: "CRE_ETH_PRIVATE_KEY",
	}
	Aptos = ChainType{
		Name:          string(corekeys.Aptos),
		PrivateKeyEnv: "CRE_APTOS_PRIVATE_KEY",
	}

	AllChainTypes = []ChainType{EVM, Aptos}
)

// Backwards-compat aliases; prefer EVM.PrivateKeyEnv / Aptos.PrivateKeyEnv.
var (
	EthPrivateKeyEnvVar   = EVM.PrivateKeyEnv
	AptosPrivateKeyEnvVar = Aptos.PrivateKeyEnv
)

// State tracked by LoadEnv / LoadPublicEnv so downstream code (e.g. build
// warnings) can inspect what happened without re-discovering or re-parsing
// the files.
var (
	loadedEnvFilePath string
	loadedEnvVars     map[string]string

	loadedPublicEnvFilePath string
	loadedPublicEnvVars     map[string]string
)

// LoadedEnvFilePath returns the .env path that was successfully loaded, or "".
func LoadedEnvFilePath() string { return loadedEnvFilePath }

// LoadedEnvVars returns the key-value pairs parsed from the loaded .env file.
// Returns nil if no file was loaded.
func LoadedEnvVars() map[string]string { return loadedEnvVars }

// LoadedPublicEnvFilePath returns the .env.public path that was successfully loaded, or "".
func LoadedPublicEnvFilePath() string { return loadedPublicEnvFilePath }

// LoadedPublicEnvVars returns the key-value pairs parsed from the loaded .env.public file.
// Returns nil if no file was loaded.
func LoadedPublicEnvVars() map[string]string { return loadedPublicEnvVars }

// Settings holds user, project, and workflow configurations.
type Settings struct {
	Workflow        WorkflowSettings
	User            UserSettings
	StorageSettings WorkflowStorageSettings
	CLDSettings     CLDSettings
}

// UserSettings stores user-specific configurations.
type UserSettings struct {
	TargetName  string
	PrivateKeys map[string]string // keyed by ChainType.Name
}

// PrivateKey returns the signing key for the given chain, or "" if unset.
func (u UserSettings) PrivateKey(f ChainType) string {
	if u.PrivateKeys == nil {
		return ""
	}
	return u.PrivateKeys[f.Name]
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

	privateKeys := make(map[string]string, len(AllChainTypes))
	for _, f := range AllChainTypes {
		privateKeys[f.Name] = NormalizeHexKey(v.GetString(f.PrivateKeyEnv))
	}

	return &Settings{
		User: UserSettings{
			TargetName:  target,
			PrivateKeys: privateKeys,
		},
		Workflow:        workflowSettings,
		StorageSettings: storageSettings,
		CLDSettings:     cldSettings,
	}, nil
}

// loadEnvFile loads the file at envPath into the process environment via
// godotenv.Overload and returns the path + parsed vars on success.
// If envPath is empty or loading fails, appropriate messages are logged
// and ("", nil) is returned.
func loadEnvFile(logger *zerolog.Logger, envPath string) (string, map[string]string) {
	if envPath == "" {
		logger.Debug().Msg(
			"No environment file specified and it was not found in the current or parent directories. " +
				"CLI tool will read individual environment variables (they MUST be exported).")
		return "", nil
	}

	if err := godotenv.Overload(envPath); err != nil {
		logger.Error().Str("path", envPath).Err(err).Msg(
			"Not able to load configuration from environment file. " +
				"CLI tool will read and verify individual environment variables (they MUST be exported). " +
				"If the file is present, please check that it follows the correct format: https://dotenvx.com/docs/env-file")
		return "", nil
	}

	vars, _ := godotenv.Read(envPath)
	return envPath, vars
}

// resolveEnvPath checks the Viper flag; if empty, auto-discovers the file by
// walking up the directory tree from the current working directory.
// Returns the resolved path and whether it was explicitly set via the CLI flag.
func resolveEnvPath(v *viper.Viper, flagName, defaultFileName string) (string, bool) {
	p := v.GetString(flagName)
	if p != "" {
		return p, true
	}
	if found, err := FindEnvFile(".", defaultFileName); err == nil {
		return found, false
	}
	return "", false
}

// LoadEnv loads environment variables from envPath into the process
// environment, then binds all loaded variables plus the sensitive defaults
// into Viper. AutomaticEnv is always activated so every OS env var is
// reachable via Viper regardless of whether a file was loaded.
// Errors are logged but do not halt execution — the CLI continues so
// that commands which don't need the env file can still run.
func LoadEnv(logger *zerolog.Logger, v *viper.Viper, envPath string) {
	loadedEnvFilePath = ""
	loadedEnvVars = nil
	loadedEnvFilePath, loadedEnvVars = loadEnvFile(logger, envPath)
	extras := []string{CreTargetEnvVar}
	for _, f := range AllChainTypes {
		extras = append(extras, f.PrivateKeyEnv)
	}
	bindAllVars(v, loadedEnvVars, extras...)
}

// LoadPublicEnv loads variables from envPath into the process environment
// and binds all loaded variables into Viper. It is intended for non-sensitive,
// shared build configuration (e.g. GOTOOLCHAIN).
func LoadPublicEnv(logger *zerolog.Logger, v *viper.Viper, envPath string) {
	loadedPublicEnvFilePath = ""
	loadedPublicEnvVars = nil
	loadedPublicEnvFilePath, loadedPublicEnvVars = loadEnvFile(logger, envPath)
	bindAllVars(v, loadedPublicEnvVars)
}

// ResolveAndLoadEnv resolves the .env file path from the given CLI flag
// (auto-detecting defaultFileName in parent dirs if the flag is empty),
// logs a debug message when the flag was not explicitly set, then loads
// the file and binds all variables into Viper.
func ResolveAndLoadEnv(logger *zerolog.Logger, v *viper.Viper, flagName, defaultFileName string) {
	path, explicit := resolveEnvPath(v, flagName, defaultFileName)
	if !explicit && path != "" {
		logger.Debug().
			Str("default", defaultFileName).
			Str("path", path).
			Msg("--env not specified; using auto-discovered file")
	}
	LoadEnv(logger, v, path)
}

// ResolveAndLoadPublicEnv resolves the public env file path from the given
// CLI flag (auto-detecting defaultFileName in parent dirs if the flag is
// empty), logs a debug message when the flag was not explicitly set, then
// loads the file and binds all variables into Viper.
func ResolveAndLoadPublicEnv(logger *zerolog.Logger, v *viper.Viper, flagName, defaultFileName string) {
	path, explicit := resolveEnvPath(v, flagName, defaultFileName)
	if !explicit && path != "" {
		logger.Debug().
			Str("default", defaultFileName).
			Str("path", path).
			Msg("--public-env not specified; using auto-discovered file")
	}
	LoadPublicEnv(logger, v, path)
}

// ResolveAndLoadBothEnvFiles resolves, loads, and binds variables from both
// the .env and .env.public files, applying the following rules:
//
//  1. If a flag is not explicitly set, a debug message is emitted; if the
//     default file is found it is loaded automatically.
//  2. Variables are prioritized: public-env > env file > other OS vars.
//     A warning is emitted for any key present in both files.
//  3. All loaded variables from both files are bound into Viper.
func ResolveAndLoadBothEnvFiles(
	logger *zerolog.Logger,
	v *viper.Viper,
	envFlagName, envDefaultFile string,
	publicEnvFlagName, publicEnvDefaultFile string,
) {
	// Load .env first (lower priority); public env loaded second overrides it.
	ResolveAndLoadEnv(logger, v, envFlagName, envDefaultFile)
	ResolveAndLoadPublicEnv(logger, v, publicEnvFlagName, publicEnvDefaultFile)

	// Rule 2: warn for keys present in both files.
	for key := range loadedPublicEnvVars {
		if _, inEnv := loadedEnvVars[key]; inEnv {
			logger.Warn().
				Str("key", key).
				Str("env", envDefaultFile).
				Str("public-env", publicEnvDefaultFile).
				Msgf("%s is defined in both env files; %s takes precedence", key, publicEnvDefaultFile)
		}
	}
}

// bindAllVars activates AutomaticEnv on v, explicitly binds every key in
// vars, and also binds any additional named keys supplied via extra.
func bindAllVars(v *viper.Viper, vars map[string]string, extra ...string) {
	v.AutomaticEnv()
	for key := range vars {
		_ = v.BindEnv(key)
	}
	for _, key := range extra {
		_ = v.BindEnv(key)
	}
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

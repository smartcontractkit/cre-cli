package settings

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type WorkflowSettings struct {
	UserWorkflowSettings struct {
		WorkflowOwnerAddress string `mapstructure:"workflow-owner-address" yaml:"workflow-owner-address"`
		WorkflowOwnerType    string `mapstructure:"workflow-owner-type" yaml:"workflow-owner-type"`
		WorkflowName         string `mapstructure:"workflow-name" yaml:"workflow-name"`
	} `mapstructure:"user-workflow" yaml:"user-workflow"`
	WorkflowArtifactSettings struct {
		WorkflowPath string `mapstructure:"workflow-path" yaml:"workflow-path"`
		ConfigPath   string `mapstructure:"config-path" yaml:"config-path"`
		SecretsPath  string `mapstructure:"secrets-path" yaml:"secrets-path"`
	} `mapstructure:"workflow-artifacts" yaml:"workflow-artifacts"`
	LoggingSettings struct {
		SethConfigPath string `mapstructure:"seth-config-path" yaml:"seth-config-path"`
	} `mapstructure:"logging" yaml:"logging"`
	RPCs []RpcEndpoint `mapstructure:"rpcs" yaml:"rpcs"`
}

func loadWorkflowSettings(logger *zerolog.Logger, v *viper.Viper, cmd *cobra.Command) (WorkflowSettings, error) {
	target, err := GetTarget(v)
	if err != nil {
		return WorkflowSettings{}, err
	}

	if !v.IsSet(target) {
		return WorkflowSettings{}, fmt.Errorf("target not found: %s", target)
	}

	getSetting := func(settingsKey string) string {
		keyWithTarget := fmt.Sprintf("%s.%s", target, settingsKey)
		if !v.IsSet(keyWithTarget) {
			logger.Debug().Msgf("setting %q not found in target %q", settingsKey, target)
			return ""
		}
		return v.GetString(keyWithTarget)
	}

	var workflowSettings WorkflowSettings

	// if a command doesn't need private key, skip getting owner here
	if !ShouldSkipGetOwner(cmd) {
		ownerAddress, ownerType, err := GetWorkflowOwner(v)
		if err != nil {
			return WorkflowSettings{}, err
		}
		workflowSettings.UserWorkflowSettings.WorkflowOwnerAddress = ownerAddress
		workflowSettings.UserWorkflowSettings.WorkflowOwnerType = ownerType
	}

	workflowSettings.UserWorkflowSettings.WorkflowName = getSetting(WorkflowNameSettingName)
	workflowSettings.WorkflowArtifactSettings.WorkflowPath = getSetting(WorkflowPathSettingName)
	workflowSettings.WorkflowArtifactSettings.ConfigPath = getSetting(ConfigPathSettingName)
	workflowSettings.WorkflowArtifactSettings.SecretsPath = getSetting(SecretsPathSettingName)
	workflowSettings.LoggingSettings.SethConfigPath = getSetting(SethConfigPathSettingName)

	fullRPCsKey := fmt.Sprintf("%s.%s", target, RpcsSettingName)
	if v.IsSet(fullRPCsKey) {
		if err := v.UnmarshalKey(fullRPCsKey, &workflowSettings.RPCs); err != nil {
			logger.Debug().Err(err).Msg("failed to unmarshal rpcs")
		}
	} else {
		logger.Debug().Msgf("rpcs settings not found in target %q", target)
	}

	if err := validateSettings(&workflowSettings); err != nil {
		return WorkflowSettings{}, err
	}

	// Validate artifact paths
	if err := validateArtifactPaths(&workflowSettings, target); err != nil {
		return WorkflowSettings{}, err
	}

	// This is required because some commands still read values directly out of viper
	// TODO: Remove this function once all access to settings no longer uses viper
	// DEVSVCS-1561
	if err := flattenWorkflowSettingsToViper(v, target); err != nil {
		return WorkflowSettings{}, err
	}

	return workflowSettings, nil
}

// TODO: Remove this function once all access to settings no longer uses viper
// DEVSVCS-1561
func flattenWorkflowSettingsToViper(v *viper.Viper, target string) error {
	// Manually flatten the workflow owner setting.
	ownerKey := fmt.Sprintf("%s.%s", target, WorkflowOwnerSettingName)
	if v.IsSet(ownerKey) {
		owner := v.GetString(ownerKey)
		v.Set(WorkflowOwnerSettingName, owner)
	}

	// Manually flatten the workflow name setting.
	wfNameKey := fmt.Sprintf("%s.%s", target, WorkflowNameSettingName)
	if v.IsSet(wfNameKey) {
		wfName := v.GetString(wfNameKey)
		v.Set(WorkflowNameSettingName, wfName)
	}

	// Manually flatten the Seth config path setting.
	sethPathKey := fmt.Sprintf("%s.%s", target, SethConfigPathSettingName)
	if v.IsSet(sethPathKey) {
		sethPath := v.GetString(sethPathKey)
		v.Set(SethConfigPathSettingName, sethPath)
	}

	// Manually flatten contracts.
	contractsKey := fmt.Sprintf("%s.%s", target, "contracts")
	if v.IsSet(contractsKey) {
		contracts := v.Get(contractsKey)
		v.Set("contracts", contracts)
	}

	// Manually flatten the RPCs setting.
	rpcsKey := fmt.Sprintf("%s.%s", target, RpcsSettingName)
	if v.IsSet(rpcsKey) {
		rpcs := v.Get(rpcsKey)
		v.Set(RpcsSettingName, rpcs)
	}

	return nil
}

func validateSettings(config *WorkflowSettings) error {
	// TODO validate that all chain names mentioned for the contracts above have a matching URL specified
	for _, rpc := range config.RPCs {
		if err := isValidRpcUrl(rpc.Url); err != nil {
			return err
		}
		if err := IsValidChainName(rpc.ChainName); err != nil {
			return err
		}
	}
	return nil
}

func isValidRpcUrl(rpcURL string) error {
	parsedURL, err := url.Parse(rpcURL)
	if err != nil {
		return fmt.Errorf("failed to parse RPC URL %s", rpcURL)
	}

	// Check if the URL has a valid scheme and host
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid scheme in RPC URL %s", rpcURL)
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("invalid host in RPC URL %s", rpcURL)
	}

	return nil
}

func IsValidChainName(name string) error {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) == 0 {
		return fmt.Errorf("chain name cannot be empty")
	}

	_, err := GetChainSelectorByChainName(trimmedName)
	if err != nil {
		return fmt.Errorf("invalid chain name %q: %w", trimmedName, err)
	}

	return nil
}

// For commands that don't need the private key, we skip getting the owner address.
// ShouldSkipGetOwner returns true if the command is `simulate` and
// `--broadcast` is false or not set. `cre help` should skip as well.
func ShouldSkipGetOwner(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "help":
		return true
	case "simulate":
		// Treat missing/invalid flag as false (i.e., skip).
		// If broadcast is explicitly true, don't skip.
		b, _ := cmd.Flags().GetBool("broadcast")
		return !b
	default:
		return false
	}
}

// validateArtifactPaths checks if artifact paths resolve and are accessible when exist
func validateArtifactPaths(workflowSettings *WorkflowSettings, target string) error {
	if err := validatePathIfSet(workflowSettings.WorkflowArtifactSettings.WorkflowPath, "WorkflowPath", target); err != nil {
		return err
	}
	if err := validatePathIfSet(workflowSettings.WorkflowArtifactSettings.ConfigPath, "ConfigPath", target); err != nil {
		return err
	}
	if err := validatePathIfSet(workflowSettings.WorkflowArtifactSettings.SecretsPath, "SecretsPath", target); err != nil {
		return err
	}
	return nil
}

// validatePathIfSet validates a single artifact path if it's non-empty
func validatePathIfSet(path, fieldName, target string) error {
	if path == "" {
		return nil
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s does not exist: %s (configured in your settings for target '%s')", fieldName, path, target)
		}
		return fmt.Errorf("%s is not accessible: %s (configured in your settings for target '%s'): %w", fieldName, path, target, err)
	}

	return nil
}

package settings

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	commonconfig "github.com/smartcontractkit/chainlink-common/pkg/config"
	crecontracts "github.com/smartcontractkit/chainlink/deployment/cre/contracts"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

type CLDSettings struct {
	CLDPath                   string `mapstructure:"cld-path" yaml:"cld-path"`
	Environment               string `mapstructure:"environment" yaml:"environment"`
	Domain                    string `mapstructure:"domain" yaml:"domain"`
	MergeProposals            bool   `mapstructure:"merge-proposals" yaml:"merge-proposals"`
	WorkflowRegistryQualifier string `mapstructure:"workflow-registry-qualifier" yaml:"workflow-registry-qualifier"`
	ChangesetFile             string `mapstructure:"changeset-file" yaml:"changeset-file"`
	MCMSSettings              struct {
		MinDelay          string `mapstructure:"min-delay" yaml:"min-delay"`
		MCMSAction        string `mapstructure:"mcms-action" yaml:"mcms-action"`
		OverrideRoot      bool   `mapstructure:"override-root" yaml:"override-root"`
		TimelockQualifier string `mapstructure:"timelock-qualifier" yaml:"timelock-qualifier"`
		ValidDuration     string `mapstructure:"valid-duration" yaml:"valid-duration"`
	} `mapstructure:"mcms-settings" yaml:"mcms-settings"`
}

func loadCLDSettings(logger *zerolog.Logger, v *viper.Viper, cmd *cobra.Command, registryChainName string) (CLDSettings, error) {
	target, err := GetTarget(v)
	if err != nil {
		return CLDSettings{}, err
	}

	if !v.IsSet(target) {
		return CLDSettings{}, fmt.Errorf("target not found: %s", target)
	}

	getSetting := func(settingsKey string) string {
		keyWithTarget := fmt.Sprintf("%s.%s", target, settingsKey)
		if !v.IsSet(keyWithTarget) {
			logger.Debug().Msgf("setting %q not found in target %q", settingsKey, target)
			return ""
		}
		return v.GetString(keyWithTarget)
	}
	var cldSettings CLDSettings

	isChangeset, _ := cmd.Flags().GetBool(Flags.Changeset.Name)
	changesetFileSpecified, _ := cmd.Flags().GetString(Flags.ChangesetFile.Name)
	if isChangeset {
		cldSettings.CLDPath = getSetting("cld-settings.cld-path")
		cldSettings.WorkflowRegistryQualifier = getSetting("cld-settings.workflow-registry-qualifier")
		cldSettings.Environment = getSetting("cld-settings.environment")
		cldSettings.Domain = getSetting("cld-settings.domain")
		cldSettings.MergeProposals = v.GetBool(fmt.Sprintf("%s.%s", target, "cld-settings.merge-proposals"))
		cldSettings.MCMSSettings.MCMSAction = getSetting("cld-settings.mcms-settings.mcms-action")
		cldSettings.MCMSSettings.TimelockQualifier = getSetting("cld-settings.mcms-settings.timelock-qualifier")
		cldSettings.MCMSSettings.MinDelay = getSetting("cld-settings.mcms-settings.min-delay")
		cldSettings.MCMSSettings.ValidDuration = getSetting("cld-settings.mcms-settings.valid-duration")
		cldSettings.MCMSSettings.OverrideRoot = v.GetBool(fmt.Sprintf("%s.%s", target, "cld-settings.mcms-settings.override-root"))
		if changesetFileSpecified != "" {
			cldSettings.ChangesetFile = changesetFileSpecified
		}
	}
	return cldSettings, nil
}

func GetMCMSConfig(settings *Settings, chainSelector uint64) (*crecontracts.MCMSConfig, error) {
	minDelay, err := time.ParseDuration(settings.CLDSettings.MCMSSettings.MinDelay)
	if err != nil {
		return nil, fmt.Errorf("failed to parse min delay duration: %w", err)
	}
	validDuration, err := time.ParseDuration(settings.CLDSettings.MCMSSettings.ValidDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to parse valid duration: %w", err)
	}
	mcmsAction := mcmstypes.TimelockAction(strings.ToLower(settings.CLDSettings.MCMSSettings.MCMSAction))

	return &crecontracts.MCMSConfig{
		MinDelay:     minDelay,
		MCMSAction:   mcmsAction,
		OverrideRoot: settings.CLDSettings.MCMSSettings.OverrideRoot,
		TimelockQualifierPerChain: map[uint64]string{
			chainSelector: settings.CLDSettings.MCMSSettings.TimelockQualifier,
		},
		ValidDuration: commonconfig.MustNewDuration(validDuration),
	}, nil
}

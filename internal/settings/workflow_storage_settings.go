package settings

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type WorkflowStorageSettings struct {
	CREStorage CREStorageSettings `mapstructure:"cre_storage"`
}

type CREStorageSettings struct {
	ServiceTimeout time.Duration `mapstructure:"servicetimeout"`
	HTTPTimeout    time.Duration `mapstructure:"httptimeout"`
}

func LoadWorkflowStorageSettings(logger *zerolog.Logger, v *viper.Viper) WorkflowStorageSettings {
	target, err := GetTarget(v)
	if err != nil {
		return WorkflowStorageSettings{}
	}

	var settings WorkflowStorageSettings
	storageKey := fmt.Sprintf("%s.workflow_storage", target)
	if err := v.UnmarshalKey(storageKey, &settings); err != nil {
		logger.Warn().Err(err).Msg("Failed to unmarshal workflow storage settings")
		return WorkflowStorageSettings{}
	}

	return settings
}

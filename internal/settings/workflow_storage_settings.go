package settings

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/gist"
)

type WorkflowStorageSettings struct {
	Gist       GistStorageSettings  `mapstructure:"gist"`
	Minio      MinioStorageSettings `mapstructure:"minio"`
	S3         S3StorageSettings    `mapstructure:"s3"`
	CREStorage CREStorageSettings   `mapstructure:"cre_storage"`
}

type GistStorageSettings struct {
	GithubToken gist.GitHubAPIToken `mapstructure:"github_token"`
}

type MinioStorageSettings struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	SessionToken    string `mapstructure:"session_token"`
	UseSSL          bool   `mapstructure:"use_ssl"`
	Region          string `mapstructure:"region"`
}

type S3StorageSettings struct {
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	SessionToken    string `mapstructure:"session_token"`
	Region          string `mapstructure:"region"`
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

	// Resolve gist environment variable placeholders
	settings.Gist.GithubToken = gist.GitHubAPIToken(resolveEnvVars(string(settings.Gist.GithubToken)))

	// Resolve minio environment variable placeholders
	settings.Minio.AccessKeyID = resolveEnvVars(settings.Minio.AccessKeyID)
	settings.Minio.SecretAccessKey = resolveEnvVars(settings.Minio.SecretAccessKey)
	settings.Minio.SessionToken = resolveEnvVars(settings.Minio.SessionToken)
	settings.Minio.Endpoint = resolveEnvVars(settings.Minio.Endpoint)
	settings.Minio.Region = resolveEnvVars(settings.Minio.Region)

	// Resolve s3 environment variable placeholders
	settings.S3.AccessKeyID = resolveEnvVars(settings.S3.AccessKeyID)
	settings.S3.SecretAccessKey = resolveEnvVars(settings.S3.SecretAccessKey)
	settings.S3.SessionToken = resolveEnvVars(settings.S3.SessionToken)
	settings.S3.Region = resolveEnvVars(settings.S3.Region)

	return settings
}

func resolveEnvVars(value string) string {
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		envVar := strings.Trim(value, "${}")
		return os.Getenv(envVar)
	}
	return value
}

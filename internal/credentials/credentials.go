package credentials

import (
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"
)

type CreLoginTokenSet struct {
	AccessToken  string `json:"access_token"  yaml:"AccessToken"`
	IDToken      string `json:"id_token"      yaml:"IDToken"`
	RefreshToken string `json:"refresh_token" yaml:"RefreshToken"`
	ExpiresIn    int    `json:"expires_in"    yaml:"ExpiresIn"`
	TokenType    string `json:"token_type"    yaml:"TokenType"`
}

type Credentials struct {
	Tokens   *CreLoginTokenSet `yaml:"tokens"`
	APIKey   string            `yaml:"api_key"`
	AuthType string            `yaml:"auth_type"`
}

const (
	CreApiKeyVar   = "CRE_API_KEY"
	AuthTypeApiKey = "api-key"
	AuthTypeBearer = "bearer"
	ConfigDir      = ".cre"
	ConfigFile     = "cre.yaml"
)

func New(logger *zerolog.Logger) (*Credentials, error) {
	cfg := &Credentials{AuthType: AuthTypeBearer}
	if key := os.Getenv(CreApiKeyVar); key != "" {
		cfg.APIKey = key
		cfg.AuthType = AuthTypeApiKey
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}
	path := filepath.Join(home, ConfigDir, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Debug().Msg("you are not logged in, try running cre login")
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, &cfg.Tokens); err != nil {
		return nil, err
	}
	return cfg, nil
}

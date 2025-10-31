package credentials

import (
	"fmt"
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
	Tokens      *CreLoginTokenSet `yaml:"tokens"`
	APIKey      string            `yaml:"api_key"`
	AuthType    string            `yaml:"auth_type"`
	IsValidated bool              `yaml:"-"`
	log         *zerolog.Logger
}

const (
	CreApiKeyVar   = "CRE_API_KEY"
	AuthTypeApiKey = "api-key"
	AuthTypeBearer = "bearer"
	ConfigDir      = ".cre"
	ConfigFile     = "cre.yaml"
)

func New(logger *zerolog.Logger) (*Credentials, error) {
	cfg := &Credentials{
		AuthType: AuthTypeBearer,
		log:      logger,
	}
	if key := os.Getenv(CreApiKeyVar); key != "" {
		cfg.APIKey = key
		cfg.AuthType = AuthTypeApiKey
		return cfg, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}
	path := filepath.Join(home, ConfigDir, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("you are not logged in, try running cre login")
	}

	if err := yaml.Unmarshal(data, &cfg.Tokens); err != nil {
		return nil, err
	}
	if cfg.Tokens == nil || cfg.Tokens.AccessToken == "" {
		return nil, fmt.Errorf("you are not logged in, try running cre login")
	}
	return cfg, nil
}

func SaveCredentials(tokenSet *CreLoginTokenSet) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, ConfigDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	path := filepath.Join(dir, ConfigFile)
	data, err := yaml.Marshal(tokenSet)
	if err != nil {
		return fmt.Errorf("marshal token set: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename temp file %s to %s: %w", tmp, path, err)
	}
	return nil
}

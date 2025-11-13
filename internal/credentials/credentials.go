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
	Tokens        *CreLoginTokenSet `yaml:"tokens"`
	APIKey        string            `yaml:"api_key"`
	AuthType      string            `yaml:"auth_type"`
	EthPrivateKey string            `yaml:"-"` // Eth private key from profile
	ProfileName   string            `yaml:"-"` // Current profile name
	IsValidated   bool              `yaml:"-"`
	log           *zerolog.Logger
}

const (
	CreApiKeyVar   = "CRE_API_KEY"
	CREProfileVar  = "CRE_PROFILE"
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

	// Check for environment variable override
	if profileName := os.Getenv(CREProfileVar); profileName != "" {
		cfg.ProfileName = profileName
	}

	// Try to load from profiles
	if profileName := os.Getenv(CREProfileVar); profileName != "" {
		if err := loadFromProfile(cfg, profileName, logger); err == nil {
			return cfg, nil
		}
		// If profile load fails but env var is set, return error
		return nil, fmt.Errorf("profile '%s' not found or invalid, run cre profile list to see available profiles", profileName)
	}

	// Try to load from profiles (use active profile)
	if err := loadFromProfile(cfg, "", logger); err == nil {
		return cfg, nil
	}

	// Fallback to legacy single-file format for backwards compatibility
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}
	path := filepath.Join(home, ConfigDir, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("you are not logged in, run cre login and try again")
	}

	if err := yaml.Unmarshal(data, &cfg.Tokens); err != nil {
		return nil, err
	}
	if cfg.Tokens == nil || cfg.Tokens.AccessToken == "" {
		return nil, fmt.Errorf("you are not logged in, run cre login and try again")
	}
	return cfg, nil
}

func loadFromProfile(cfg *Credentials, profileName string, logger *zerolog.Logger) error {
	// Dynamically import profiles to avoid circular dependency
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	profilesPath := filepath.Join(home, ConfigDir, "profiles.yaml")
	data, err := os.ReadFile(profilesPath)
	if err != nil {
		return err // profiles.yaml doesn't exist
	}

	var profilesConfig struct {
		Version       string `yaml:"version"`
		ActiveProfile string `yaml:"active_profile"`
		Profiles      []struct {
			Name          string            `yaml:"name"`
			Tokens        *CreLoginTokenSet `yaml:"tokens,omitempty"`
			APIKey        string            `yaml:"api_key,omitempty"`
			AuthType      string            `yaml:"auth_type,omitempty"`
			EthPrivateKey string            `yaml:"eth_private_key,omitempty"`
		} `yaml:"profiles"`
	}

	if err := yaml.Unmarshal(data, &profilesConfig); err != nil {
		return err
	}

	// Determine which profile to use
	targetProfile := profileName
	if targetProfile == "" {
		targetProfile = profilesConfig.ActiveProfile
	}

	if targetProfile == "" && len(profilesConfig.Profiles) > 0 {
		// Use first profile if no active profile specified
		targetProfile = profilesConfig.Profiles[0].Name
	}

	if targetProfile == "" {
		return fmt.Errorf("no profile specified and no active profile found")
	}

	// Find the profile
	for _, profile := range profilesConfig.Profiles {
		if profile.Name == targetProfile {
			cfg.ProfileName = profile.Name
			cfg.Tokens = profile.Tokens
			cfg.APIKey = profile.APIKey
			if profile.AuthType != "" {
				cfg.AuthType = profile.AuthType
			}
			if profile.EthPrivateKey != "" {
				cfg.EthPrivateKey = profile.EthPrivateKey
			}
			return nil
		}
	}

	return fmt.Errorf("profile '%s' not found", targetProfile)
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

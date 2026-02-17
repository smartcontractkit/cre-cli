package credentials

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"
)

type CreLoginTokenSet struct {
	AccessToken  string `json:"access_token"  yaml:"AccessToken"` // #nosec G117 -- matches OAuth token response field
	IDToken      string `json:"id_token"      yaml:"IDToken"`
	RefreshToken string `json:"refresh_token" yaml:"RefreshToken"` // #nosec G117 -- matches OAuth token response field
	ExpiresIn    int    `json:"expires_in"    yaml:"ExpiresIn"`
	TokenType    string `json:"token_type"    yaml:"TokenType"`
}

type Credentials struct {
	Tokens      *CreLoginTokenSet `yaml:"tokens"`
	APIKey      string            `yaml:"api_key"` // #nosec G117 -- credential stored in secure config file
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

// UngatedOrgRequiredMsg is the error message shown when an organization does not have ungated access.
var UngatedOrgRequiredMsg = "\n✖ Workflow deployment is currently in early access. We're onboarding organizations gradually.\n\nWant to deploy?\n→ Request access here: https://cre.chain.link/request-access\n"

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

// CheckIsUngatedOrganization verifies that the organization associated with the credentials
// has FULL_ACCESS status (is not gated). This check is required for certain operations like
// workflow key linking.
func (c *Credentials) CheckIsUngatedOrganization() error {
	// API keys can only be generated on ungated organizations, so they always pass
	if c.AuthType == AuthTypeApiKey {
		return nil
	}

	// For JWT bearer tokens, we need to parse the token and check the organization_status claim
	if c.Tokens == nil || c.Tokens.AccessToken == "" {
		return fmt.Errorf("no access token available")
	}

	// Parse the JWT to extract claims
	parts := strings.Split(c.Tokens.AccessToken, ".")
	if len(parts) < 2 {
		return fmt.Errorf("invalid JWT token format")
	}

	// Decode the payload (second part of the JWT)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	// Parse claims into a map
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return fmt.Errorf("failed to unmarshal JWT claims: %w", err)
	}

	// Log all claims for debugging
	c.log.Debug().Interface("claims", claims).Msg("JWT claims decoded")

	// Dynamically find the organization_status claim by looking for any key ending with "organization_status"
	var orgStatus string
	var orgStatusKey string
	for key, value := range claims {
		if strings.HasSuffix(key, "organization_status") {
			if status, ok := value.(string); ok {
				orgStatus = status
				orgStatusKey = key
				break
			}
		}
	}

	c.log.Debug().Str("claim_key", orgStatusKey).Str("organization_status", orgStatus).Msg("checking organization status claim")

	if orgStatus == "" {
		// If the claim is missing or empty, the organization is considered gated
		return errors.New(UngatedOrgRequiredMsg)
	}

	// Check if the organization has full access
	if orgStatus != "FULL_ACCESS" {
		c.log.Debug().Str("organization_status", orgStatus).Msg("organization does not have FULL_ACCESS - organization is gated")
		return errors.New(UngatedOrgRequiredMsg)
	}
	c.log.Debug().Str("organization_status", orgStatus).Msg("organization has FULL_ACCESS - organization is ungated")

	return nil
}

package profiles

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
)

const (
	ConfigDir    = ".cre"
	ProfilesFile = "profiles.yaml"
	LegacyFile   = "cre.yaml" // for backwards compatibility
)

// Profile represents a single account/organization profile
type Profile struct {
	Name      string                        `yaml:"name"`
	Org       string                        `yaml:"org,omitempty"` // organization name for reference
	OrgID     string                        `yaml:"org_id,omitempty"`
	Email     string                        `yaml:"email,omitempty"`
	Tokens    *credentials.CreLoginTokenSet `yaml:"tokens,omitempty"`
	APIKey    string                        `yaml:"api_key,omitempty"`
	AuthType  string                        `yaml:"auth_type,omitempty"`
	CreatedAt string                        `yaml:"created_at,omitempty"`
	UpdatedAt string                        `yaml:"updated_at,omitempty"`
}

// ProfilesConfig represents the entire profiles configuration
type ProfilesConfig struct {
	Version       string     `yaml:"version"`
	ActiveProfile string     `yaml:"active_profile"`
	Profiles      []*Profile `yaml:"profiles"`
}

// Manager handles profile operations
type Manager struct {
	configPath string
	log        *zerolog.Logger
	config     *ProfilesConfig
}

// New creates a new profile manager
func New(logger *zerolog.Logger) (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	configPath := filepath.Join(home, ConfigDir, ProfilesFile)
	m := &Manager{
		configPath: configPath,
		log:        logger,
	}

	// Try to load existing config
	if err := m.load(); err != nil {
		// If file doesn't exist, initialize with empty config
		if os.IsNotExist(err) {
			m.config = &ProfilesConfig{
				Version:       "1.0",
				ActiveProfile: "",
				Profiles:      []*Profile{},
			}
			// Try to migrate from legacy single-file format
			if err := m.migrateFromLegacy(); err != nil {
				m.log.Debug().Err(err).Msg("no legacy credentials to migrate")
			}
		} else {
			return nil, fmt.Errorf("failed to load profiles: %w", err)
		}
	}

	return m, nil
}

// migrateFromLegacy migrates credentials from the old single-file format
func (m *Manager) migrateFromLegacy() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	legacyPath := filepath.Join(home, ConfigDir, LegacyFile)
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return err // File doesn't exist or can't be read
	}

	// Try to unmarshal as token set
	var tokens credentials.CreLoginTokenSet
	if err := yaml.Unmarshal(data, &tokens); err != nil {
		return err
	}

	if tokens.AccessToken == "" {
		return fmt.Errorf("invalid legacy credentials")
	}

	// Create default profile from legacy credentials
	profile := &Profile{
		Name:     "default",
		Tokens:   &tokens,
		AuthType: credentials.AuthTypeBearer,
	}

	m.config.Profiles = append(m.config.Profiles, profile)
	m.config.ActiveProfile = "default"

	// Save the migrated config
	if err := m.save(); err != nil {
		return fmt.Errorf("failed to save migrated profiles: %w", err)
	}

	m.log.Info().Msg("successfully migrated credentials from legacy format")
	return nil
}

// load reads the profiles configuration from disk
func (m *Manager) load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	config := &ProfilesConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return err
	}

	m.config = config
	return nil
}

// save writes the profiles configuration to disk
func (m *Manager) save() error {
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("marshal profiles: %w", err)
	}

	tmp := m.configPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmp, m.configPath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// SaveProfile saves or updates a profile
func (m *Manager) SaveProfile(profile *Profile) error {
	if profile.Name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	// Check if profile already exists
	for i, p := range m.config.Profiles {
		if p.Name == profile.Name {
			// Update existing profile
			m.config.Profiles[i] = profile
			return m.save()
		}
	}

	// Add new profile
	m.config.Profiles = append(m.config.Profiles, profile)

	// If this is the first profile, make it active
	if m.config.ActiveProfile == "" {
		m.config.ActiveProfile = profile.Name
	}

	return m.save()
}

// GetProfile retrieves a profile by name
func (m *Manager) GetProfile(name string) *Profile {
	for _, p := range m.config.Profiles {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// GetActiveProfile retrieves the currently active profile
func (m *Manager) GetActiveProfile() *Profile {
	if m.config.ActiveProfile == "" {
		return nil
	}
	return m.GetProfile(m.config.ActiveProfile)
}

// SetActiveProfile sets the active profile
func (m *Manager) SetActiveProfile(name string) error {
	profile := m.GetProfile(name)
	if profile == nil {
		return fmt.Errorf("profile '%s' not found", name)
	}

	m.config.ActiveProfile = name
	return m.save()
}

// ListProfiles returns all profiles
func (m *Manager) ListProfiles() []*Profile {
	return m.config.Profiles
}

// DeleteProfile removes a profile
func (m *Manager) DeleteProfile(name string) error {
	for i, p := range m.config.Profiles {
		if p.Name == name {
			m.config.Profiles = append(m.config.Profiles[:i], m.config.Profiles[i+1:]...)

			// If we deleted the active profile, switch to the first available
			if m.config.ActiveProfile == name {
				if len(m.config.Profiles) > 0 {
					m.config.ActiveProfile = m.config.Profiles[0].Name
				} else {
					m.config.ActiveProfile = ""
				}
			}

			return m.save()
		}
	}

	return fmt.Errorf("profile '%s' not found", name)
}

// RenameProfile renames an existing profile
func (m *Manager) RenameProfile(oldName, newName string) error {
	if newName == "" {
		return fmt.Errorf("new profile name cannot be empty")
	}

	// Check if new name already exists
	if m.GetProfile(newName) != nil {
		return fmt.Errorf("profile '%s' already exists", newName)
	}

	profile := m.GetProfile(oldName)
	if profile == nil {
		return fmt.Errorf("profile '%s' not found", oldName)
	}

	profile.Name = newName

	// If this was the active profile, update the active profile name
	if m.config.ActiveProfile == oldName {
		m.config.ActiveProfile = newName
	}

	return m.save()
}

// GetProfileByOrgID retrieves a profile by organization ID
func (m *Manager) GetProfileByOrgID(orgID string) *Profile {
	for _, p := range m.config.Profiles {
		if p.OrgID == orgID {
			return p
		}
	}
	return nil
}

// GetActiveProfileName returns the name of the active profile
func (m *Manager) GetActiveProfileName() string {
	return m.config.ActiveProfile
}

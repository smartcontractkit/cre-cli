package profiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
)

const (
	ConfigDir      = ".cre"
	ProfilesFolder = "profiles" // Directory: individual profile files
	LegacyFile     = "cre.yaml" // Legacy: backwards compatibility
)

// Profile represents a single account/organization profile
type Profile struct {
	Name          string                        `yaml:"name"`
	Org           string                        `yaml:"org,omitempty"` // organization name for reference
	OrgID         string                        `yaml:"org_id,omitempty"`
	Email         string                        `yaml:"email,omitempty"`
	Tokens        *credentials.CreLoginTokenSet `yaml:"tokens,omitempty"`
	APIKey        string                        `yaml:"api_key,omitempty"`
	AuthType      string                        `yaml:"auth_type,omitempty"`
	EthPrivateKey string                        `yaml:"eth_private_key,omitempty"`
	CreatedAt     string                        `yaml:"created_at,omitempty"`
	UpdatedAt     string                        `yaml:"updated_at,omitempty"`
}

// Manager handles profile operations
type Manager struct {
	profilesDir string
	log         *zerolog.Logger
}

// New creates a new profile manager
func New(logger *zerolog.Logger) (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	profilesDir := filepath.Join(home, ConfigDir, ProfilesFolder)
	m := &Manager{
		profilesDir: profilesDir,
		log:         logger,
	}

	// Try to migrate from legacy single-file format on first run
	if err := m.migrateFromLegacy(); err != nil {
		m.log.Debug().Err(err).Msg("no legacy credentials to migrate")
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

	// Save the migrated profile
	if err := m.SaveProfile(profile); err != nil {
		return fmt.Errorf("failed to save migrated profile: %w", err)
	}

	m.log.Info().Msg("successfully migrated credentials from legacy format")
	return nil
}

// getProfilePath returns the file path for a specific profile
func (m *Manager) getProfilePath(profileName string) string {
	return filepath.Join(m.profilesDir, profileName+".yaml")
}

// SaveProfile saves or updates a profile
func (m *Manager) SaveProfile(profile *Profile) error {
	if profile.Name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	// Ensure profiles directory exists
	if err := os.MkdirAll(m.profilesDir, 0o700); err != nil {
		return fmt.Errorf("create profiles directory: %w", err)
	}

	// Save individual profile to its own file
	profilePath := m.getProfilePath(profile.Name)
	data, err := yaml.Marshal(profile)
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}

	tmp := profilePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write profile temp file: %w", err)
	}

	if err := os.Rename(tmp, profilePath); err != nil {
		return fmt.Errorf("rename profile file: %w", err)
	}

	return nil
}

// GetProfile retrieves a profile by name from individual file
func (m *Manager) GetProfile(name string) *Profile {
	profilePath := m.getProfilePath(name)
	data, err := os.ReadFile(profilePath)
	if err != nil {
		m.log.Debug().Err(err).Str("profile", name).Msg("failed to read profile file")
		return nil
	}

	var profile Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		m.log.Debug().Err(err).Str("profile", name).Msg("failed to unmarshal profile")
		return nil
	}
	return &profile
}

// GetActiveProfile retrieves the active profile from CRE_PROFILE env var
func (m *Manager) GetActiveProfile() *Profile {
	activeProfileName := os.Getenv(credentials.CREProfileVar)
	if activeProfileName == "" {
		return nil
	}
	return m.GetProfile(activeProfileName)
}

// SetActiveProfile sets the active profile via CRE_PROFILE env var
func (m *Manager) SetActiveProfile(name string) error {
	profile := m.GetProfile(name)
	if profile == nil {
		return fmt.Errorf("profile '%s' not found", name)
	}

	return os.Setenv(credentials.CREProfileVar, name)
}

// ListProfiles returns all profiles by reading from individual files
func (m *Manager) ListProfiles() []*Profile {
	entries, err := os.ReadDir(m.profilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Profile{}
		}
		m.log.Debug().Err(err).Msg("failed to read profiles directory")
		return []*Profile{}
	}

	var profiles []*Profile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		profileName := strings.TrimSuffix(entry.Name(), ".yaml")
		if profile := m.GetProfile(profileName); profile != nil {
			profiles = append(profiles, profile)
		}
	}

	return profiles
}

// DeleteProfile removes a profile and its file
func (m *Manager) DeleteProfile(name string) error {
	// Check if profile exists
	if m.GetProfile(name) == nil {
		return fmt.Errorf("profile '%s' not found", name)
	}

	// Delete the individual profile file
	profilePath := m.getProfilePath(name)
	if err := os.Remove(profilePath); err != nil && !os.IsNotExist(err) {
		m.log.Warn().Err(err).Str("profile", name).Msg("failed to delete profile file")
	}

	// If we deleted the active profile, clear it from env var
	if os.Getenv(credentials.CREProfileVar) == name {
		os.Unsetenv(credentials.CREProfileVar)
	}

	return nil
}

// RenameProfile renames an existing profile and its file
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

	// Update profile name
	profile.Name = newName

	// Save to new file name
	if err := m.SaveProfile(profile); err != nil {
		return fmt.Errorf("failed to save renamed profile: %w", err)
	}

	// Delete old file
	oldPath := m.getProfilePath(oldName)
	if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
		m.log.Warn().Err(err).Str("profile", oldName).Msg("failed to delete old profile file")
	}

	// Update active profile env var if applicable
	if os.Getenv(credentials.CREProfileVar) == oldName {
		os.Setenv(credentials.CREProfileVar, newName)
	}

	return nil
}

// GetProfileByOrgID retrieves a profile by organization ID
func (m *Manager) GetProfileByOrgID(orgID string) *Profile {
	// Search all profiles by loading from individual files
	profiles := m.ListProfiles()
	for _, p := range profiles {
		if p.OrgID == orgID {
			return p
		}
	}
	return nil
}

// GetActiveProfileName returns the name of the active profile from env var
func (m *Manager) GetActiveProfileName() string {
	return os.Getenv(credentials.CREProfileVar)
}

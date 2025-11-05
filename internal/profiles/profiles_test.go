package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
)

func TestProfileManager(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	// Create temp home structure
	creDir := filepath.Join(tmpDir, ".cre")
	if err := os.MkdirAll(creDir, 0o700); err != nil {
		t.Fatalf("failed to create temp .cre dir: %v", err)
	}
	os.Setenv("HOME", tmpDir)

	logger := zerolog.New(os.Stderr)

	t.Run("create and list profiles", func(t *testing.T) {
		manager, err := New(&logger)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		profile1 := &Profile{
			Name:     "org1",
			Org:      "Organization 1",
			OrgID:    "org-id-1",
			Email:    "user@org1.com",
			AuthType: credentials.AuthTypeBearer,
			Tokens: &credentials.CreLoginTokenSet{
				AccessToken:  "token1",
				RefreshToken: "refresh1",
				ExpiresIn:    3600,
				TokenType:    "Bearer",
			},
		}

		if err := manager.SaveProfile(profile1); err != nil {
			t.Fatalf("failed to save profile: %v", err)
		}

		profiles := manager.ListProfiles()
		if len(profiles) != 1 {
			t.Errorf("expected 1 profile, got %d", len(profiles))
		}

		if profiles[0].Name != "org1" {
			t.Errorf("expected profile name 'org1', got %q", profiles[0].Name)
		}
	})

	t.Run("set and get active profile", func(t *testing.T) {
		manager, err := New(&logger)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		profile1 := &Profile{
			Name:     "profile1",
			Org:      "Org 1",
			AuthType: credentials.AuthTypeBearer,
		}
		profile2 := &Profile{
			Name:     "profile2",
			Org:      "Org 2",
			AuthType: credentials.AuthTypeBearer,
		}

		if err := manager.SaveProfile(profile1); err != nil {
			t.Fatalf("failed to save profile1: %v", err)
		}
		if err := manager.SaveProfile(profile2); err != nil {
			t.Fatalf("failed to save profile2: %v", err)
		}

		if err := manager.SetActiveProfile("profile2"); err != nil {
			t.Fatalf("failed to set active profile: %v", err)
		}

		active := manager.GetActiveProfile()
		if active == nil {
			t.Fatal("expected active profile, got nil")
		}
		if active.Name != "profile2" {
			t.Errorf("expected active profile 'profile2', got %q", active.Name)
		}
	})

	t.Run("rename profile", func(t *testing.T) {
		manager, err := New(&logger)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		profile := &Profile{
			Name:     "oldname",
			Org:      "Organization",
			AuthType: credentials.AuthTypeBearer,
		}

		if err := manager.SaveProfile(profile); err != nil {
			t.Fatalf("failed to save profile: %v", err)
		}

		if err := manager.RenameProfile("oldname", "newname"); err != nil {
			t.Fatalf("failed to rename profile: %v", err)
		}

		profile = manager.GetProfile("newname")
		if profile == nil {
			t.Fatal("expected renamed profile, got nil")
		}

		oldProfile := manager.GetProfile("oldname")
		if oldProfile != nil {
			t.Fatal("expected old profile to be gone, but found it")
		}
	})

	t.Run("delete profile", func(t *testing.T) {
		manager, err := New(&logger)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		profile := &Profile{
			Name:     "todelete",
			Org:      "Organization",
			AuthType: credentials.AuthTypeBearer,
		}

		if err := manager.SaveProfile(profile); err != nil {
			t.Fatalf("failed to save profile: %v", err)
		}

		if err := manager.DeleteProfile("todelete"); err != nil {
			t.Fatalf("failed to delete profile: %v", err)
		}

		profile = manager.GetProfile("todelete")
		if profile != nil {
			t.Fatal("expected profile to be deleted, but found it")
		}
	})
}


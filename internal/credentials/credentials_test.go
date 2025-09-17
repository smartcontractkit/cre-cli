package credentials

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func TestNew_Default(t *testing.T) {
	t.Setenv(CreApiKeyVar, "")
	t.Setenv("HOME", t.TempDir())
	logger := testutil.NewTestLogger()

	cfg, err := New(logger)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", cfg.APIKey)
	}
	if cfg.AuthType != AuthTypeBearer {
		t.Errorf("expected AuthType %q, got %q", AuthTypeBearer, cfg.AuthType)
	}
	if cfg.Tokens != nil {
		t.Error("expected nil Tokens when no config file present")
	}
}

func TestNew_WithEnvAPIKey(t *testing.T) {
	t.Setenv(CreApiKeyVar, "env-key")
	t.Setenv("HOME", t.TempDir())
	logger := testutil.NewTestLogger()

	cfg, err := New(logger)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("expected APIKey %q, got %q", "env-key", cfg.APIKey)
	}
	if cfg.AuthType != AuthTypeApiKey {
		t.Errorf("expected AuthType %q, got %q", AuthTypeApiKey, cfg.AuthType)
	}
}
func TestNew_WithConfigFile(t *testing.T) {
	t.Setenv(CreApiKeyVar, "")
	tDir := t.TempDir()
	t.Setenv("HOME", tDir)

	dir := filepath.Join(tDir, ConfigDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	file := filepath.Join(dir, ConfigFile)
	content := `AccessToken: "file-token"
IDToken: "id-token"
RefreshToken: "refresh-token"
ExpiresIn:  99
TokenType:  "file-type"
`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	logger := testutil.NewTestLogger()

	cfg, err := New(logger)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := cfg.Tokens.AccessToken; got != "file-token" {
		t.Errorf("expected AccessToken %q, got %q", "file-token", got)
	}
	if got := cfg.Tokens.IDToken; got != "id-token" {
		t.Errorf("expected IDToken %q, got %q", "id-token", got)
	}
	if got := cfg.Tokens.RefreshToken; got != "refresh-token" {
		t.Errorf("expected RefreshToken %q, got %q", "refresh-token", got)
	}
	if got := cfg.Tokens.ExpiresIn; got != 99 {
		t.Errorf("expected ExpiresIn %d, got %d", 99, got)
	}
	if got := cfg.Tokens.TokenType; got != "file-type" {
		t.Errorf("expected TokenType %q, got %q", "file-type", got)
	}
	if cfg.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", cfg.APIKey)
	}
	if cfg.AuthType != AuthTypeBearer {
		t.Errorf("expected AuthType %q, got %q", AuthTypeBearer, cfg.AuthType)
	}
}

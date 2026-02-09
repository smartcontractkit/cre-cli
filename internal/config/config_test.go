package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func TestParseRepoString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"owner/repo@main", "owner/repo@main", false},
		{"owner/repo@v1.0.0", "owner/repo@v1.0.0", false},
		{"owner/repo", "owner/repo@main", false},
		{"org/my-templates@feature/branch", "org/my-templates@feature/branch", false},
		{"invalid", "", true},
		{"/repo@main", "", true},
		{"owner/@main", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			source, err := ParseRepoString(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, source.String())
			}
		})
	}
}

func TestLoadTemplateSourcesDefault(t *testing.T) {
	logger := testutil.NewTestLogger()

	// Ensure env var is not set
	t.Setenv("CRE_TEMPLATE_REPOS", "")

	sources := LoadTemplateSources(logger)
	require.Len(t, sources, 1)
	assert.Equal(t, "smartcontractkit", sources[0].Owner)
	assert.Equal(t, "cre-templates", sources[0].Repo)
}

func TestLoadTemplateSourcesFromEnv(t *testing.T) {
	logger := testutil.NewTestLogger()

	t.Setenv("CRE_TEMPLATE_REPOS", "org1/repo1@main,org2/repo2@v1.0")

	sources := LoadTemplateSources(logger)
	require.Len(t, sources, 2)
	assert.Equal(t, "org1", sources[0].Owner)
	assert.Equal(t, "repo1", sources[0].Repo)
	assert.Equal(t, "org2", sources[1].Owner)
	assert.Equal(t, "v1.0", sources[1].Ref)
}

func TestLoadTemplateSourcesFromConfigFile(t *testing.T) {
	logger := testutil.NewTestLogger()

	// Ensure env var is not set
	t.Setenv("CRE_TEMPLATE_REPOS", "")

	// Create a temporary config file
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	configDir := filepath.Join(homeDir, ".cre")
	require.NoError(t, os.MkdirAll(configDir, 0750))

	configContent := `templateRepositories:
  - owner: custom-org
    repo: custom-templates
    ref: release
`
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "config.yaml"),
		[]byte(configContent),
		0600,
	))

	sources := LoadTemplateSources(logger)
	require.Len(t, sources, 1)
	assert.Equal(t, "custom-org", sources[0].Owner)
	assert.Equal(t, "custom-templates", sources[0].Repo)
	assert.Equal(t, "release", sources[0].Ref)
}

func TestEnvOverridesConfigFile(t *testing.T) {
	logger := testutil.NewTestLogger()

	t.Setenv("CRE_TEMPLATE_REPOS", "env-org/env-repo@main")

	sources := LoadTemplateSources(logger)
	require.Len(t, sources, 1)
	assert.Equal(t, "env-org", sources[0].Owner)
}

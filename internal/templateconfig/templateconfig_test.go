package templateconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/templaterepo"
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

	// Point HOME to a temp dir with no config file
	t.Setenv("HOME", t.TempDir())

	sources := LoadTemplateSources(logger)
	require.Len(t, sources, 1)
	assert.Equal(t, "smartcontractkit", sources[0].Owner)
	assert.Equal(t, "cre-templates", sources[0].Repo)
}

func TestLoadTemplateSourcesFromConfigFile(t *testing.T) {
	logger := testutil.NewTestLogger()

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
		filepath.Join(configDir, "template.yaml"),
		[]byte(configContent),
		0600,
	))

	sources := LoadTemplateSources(logger)
	require.Len(t, sources, 1)
	assert.Equal(t, "custom-org", sources[0].Owner)
	assert.Equal(t, "custom-templates", sources[0].Repo)
	assert.Equal(t, "release", sources[0].Ref)
}

func TestSaveTemplateSources(t *testing.T) {
	logger := testutil.NewTestLogger()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	sources := []templaterepo.RepoSource{
		{Owner: "org1", Repo: "repo1", Ref: "main"},
		{Owner: "org2", Repo: "repo2", Ref: "v1.0"},
	}

	require.NoError(t, SaveTemplateSources(sources))

	// Verify file exists
	configPath := filepath.Join(homeDir, ".cre", "template.yaml")
	_, err := os.Stat(configPath)
	require.NoError(t, err)

	// Verify content by loading back
	loaded := LoadTemplateSources(logger)
	require.Len(t, loaded, 2)
	assert.Equal(t, "org1", loaded[0].Owner)
	assert.Equal(t, "repo1", loaded[0].Repo)
	assert.Equal(t, "main", loaded[0].Ref)
	assert.Equal(t, "org2", loaded[1].Owner)
	assert.Equal(t, "repo2", loaded[1].Repo)
	assert.Equal(t, "v1.0", loaded[1].Ref)
}

func TestEnsureDefaultConfig(t *testing.T) {
	logger := testutil.NewTestLogger()

	t.Run("creates file when missing", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		require.NoError(t, EnsureDefaultConfig(logger))

		// File should exist with default source
		sources := LoadTemplateSources(logger)
		require.Len(t, sources, 1)
		assert.Equal(t, DefaultSource.Owner, sources[0].Owner)
		assert.Equal(t, DefaultSource.Repo, sources[0].Repo)
		assert.Equal(t, DefaultSource.Ref, sources[0].Ref)
	})

	t.Run("no-op when file exists", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		// Write custom config first
		custom := []templaterepo.RepoSource{
			{Owner: "my-org", Repo: "my-templates", Ref: "dev"},
		}
		require.NoError(t, SaveTemplateSources(custom))

		// EnsureDefaultConfig should not overwrite
		require.NoError(t, EnsureDefaultConfig(logger))

		sources := LoadTemplateSources(logger)
		require.Len(t, sources, 1)
		assert.Equal(t, "my-org", sources[0].Owner)
	})
}

func TestAddRepoToExisting(t *testing.T) {
	logger := testutil.NewTestLogger()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	// Start with default
	require.NoError(t, SaveTemplateSources([]templaterepo.RepoSource{DefaultSource}))

	// Load, append, save
	existing := LoadTemplateSources(logger)
	newRepo := templaterepo.RepoSource{Owner: "my-org", Repo: "my-templates", Ref: "main"}
	updated := append(existing, newRepo)
	require.NoError(t, SaveTemplateSources(updated))

	// Verify both are present
	final := LoadTemplateSources(logger)
	require.Len(t, final, 2)
	assert.Equal(t, DefaultSource.Owner, final[0].Owner)
	assert.Equal(t, "my-org", final[1].Owner)
}

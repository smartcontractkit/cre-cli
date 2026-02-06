package templaterepo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func TestCacheLoadSave(t *testing.T) {
	logger := testutil.NewTestLogger()
	cacheDir := t.TempDir()
	cache := NewCacheWithDir(logger, cacheDir)

	source := RepoSource{Owner: "test", Repo: "templates", Ref: "main"}

	// Initially no cache
	templates, fresh := cache.LoadTemplateList(source)
	assert.Nil(t, templates)
	assert.False(t, fresh)

	// Save some templates
	testTemplates := []TemplateSummary{
		{
			TemplateMetadata: TemplateMetadata{
				Name:     "test-go",
				Title:    "Test Go",
				Language: "go",
				Kind:     "building-block",
			},
			Path:   "building-blocks/test-go",
			Source: source,
		},
	}

	err := cache.SaveTemplateList(source, testTemplates, "sha123")
	require.NoError(t, err)

	// Load should return fresh data
	loaded, fresh := cache.LoadTemplateList(source)
	assert.True(t, fresh)
	require.Len(t, loaded, 1)
	assert.Equal(t, "test-go", loaded[0].Name)
}

func TestCacheTTLExpiry(t *testing.T) {
	logger := testutil.NewTestLogger()
	cacheDir := t.TempDir()
	cache := NewCacheWithDir(logger, cacheDir)

	source := RepoSource{Owner: "test", Repo: "templates", Ref: "main"}

	// Write cache manually with expired timestamp
	cacheData := templateListCache{
		Templates: []TemplateSummary{
			{
				TemplateMetadata: TemplateMetadata{
					Name: "old-template",
				},
				Source: source,
			},
		},
		TreeSHA:   "oldsha",
		LastCheck: time.Now().Add(-2 * time.Hour), // 2 hours ago (expired)
	}

	data, err := json.Marshal(cacheData)
	require.NoError(t, err)

	cachePath := cache.templateListPath(source)
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0750))
	require.NoError(t, os.WriteFile(cachePath, data, 0600))

	// LoadTemplateList should indicate stale
	templates, fresh := cache.LoadTemplateList(source)
	assert.False(t, fresh)
	require.Len(t, templates, 1)
	assert.Equal(t, "old-template", templates[0].Name)

	// LoadStaleTemplateList should still return data
	stale := cache.LoadStaleTemplateList(source)
	require.Len(t, stale, 1)
	assert.Equal(t, "old-template", stale[0].Name)
}

func TestCacheCorruptFile(t *testing.T) {
	logger := testutil.NewTestLogger()
	cacheDir := t.TempDir()
	cache := NewCacheWithDir(logger, cacheDir)

	source := RepoSource{Owner: "test", Repo: "templates", Ref: "main"}

	// Write corrupt data
	cachePath := cache.templateListPath(source)
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0750))
	require.NoError(t, os.WriteFile(cachePath, []byte("not json"), 0600))

	templates, fresh := cache.LoadTemplateList(source)
	assert.Nil(t, templates)
	assert.False(t, fresh)
}

func TestTarballCache(t *testing.T) {
	logger := testutil.NewTestLogger()
	cacheDir := t.TempDir()
	cache := NewCacheWithDir(logger, cacheDir)

	source := RepoSource{Owner: "test", Repo: "templates", Ref: "main"}

	// Not cached initially
	assert.False(t, cache.IsTarballCached(source, "sha123"))

	// Create a tarball file
	tarballPath := cache.TarballPath(source, "sha123")
	require.NoError(t, os.MkdirAll(filepath.Dir(tarballPath), 0750))
	require.NoError(t, os.WriteFile(tarballPath, []byte("fake tarball"), 0600))

	// Now it should be cached
	assert.True(t, cache.IsTarballCached(source, "sha123"))
}

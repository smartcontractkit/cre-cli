package templaterepo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func newTestServer(templates map[string]string) *httptest.Server {
	treeEntries := []treeEntry{}
	for path := range templates {
		treeEntries = append(treeEntries, treeEntry{Path: path, Type: "blob"})
	}

	treeResp := treeResponse{
		SHA:  "testsha123",
		Tree: treeEntries,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Tree API
		if r.URL.Query().Get("recursive") == "1" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(treeResp)
			return
		}

		// Raw content
		for path, content := range templates {
			if r.URL.Path == "/raw/"+path {
				w.Write([]byte(content))
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
	})

	return httptest.NewServer(mux)
}

func TestRegistryListTemplates(t *testing.T) {
	logger := testutil.NewTestLogger()
	cacheDir := t.TempDir()
	cache := NewCacheWithDir(logger, cacheDir)

	source := RepoSource{Owner: "test", Repo: "templates", Ref: "main"}

	// Pre-populate cache so we don't need a real GitHub API call
	testTemplates := []TemplateSummary{
		{
			TemplateMetadata: TemplateMetadata{
				Kind:        "building-block",
				Name:        "kv-store-go",
				Title:       "Key-Value Store (Go)",
				Description: "A Go KV store",
				Language:    "go",
			},
			Path:   "building-blocks/kv-store/kv-store-go",
			Source: source,
		},
		{
			TemplateMetadata: TemplateMetadata{
				Kind:        "building-block",
				Name:        "kv-store-ts",
				Title:       "Key-Value Store (TypeScript)",
				Description: "A TS KV store",
				Language:    "typescript",
			},
			Path:   "building-blocks/kv-store/kv-store-ts",
			Source: source,
		},
		{
			TemplateMetadata: TemplateMetadata{
				Kind:        "starter-template",
				Name:        "custom-feed-go",
				Title:       "Custom Data Feed (Go)",
				Description: "A custom data feed",
				Language:    "go",
			},
			Path:   "starter-templates/custom-feed/custom-feed-go",
			Source: source,
		},
	}

	err := cache.SaveTemplateList(source, testTemplates, "testsha123")
	require.NoError(t, err)

	client := NewClient(logger)
	registry := NewRegistryWithCache(logger, client, cache, []RepoSource{source})

	// List should return all cached templates
	templates, err := registry.ListTemplates(false)
	require.NoError(t, err)
	assert.Len(t, templates, 3)
}

func TestRegistryGetTemplate(t *testing.T) {
	logger := testutil.NewTestLogger()
	cacheDir := t.TempDir()
	cache := NewCacheWithDir(logger, cacheDir)

	source := RepoSource{Owner: "test", Repo: "templates", Ref: "main"}

	testTemplates := []TemplateSummary{
		{
			TemplateMetadata: TemplateMetadata{
				Name:     "kv-store-go",
				Title:    "Key-Value Store (Go)",
				Language: "go",
				Kind:     "building-block",
			},
			Path:   "building-blocks/kv-store/kv-store-go",
			Source: source,
		},
	}

	err := cache.SaveTemplateList(source, testTemplates, "sha123")
	require.NoError(t, err)

	client := NewClient(logger)
	registry := NewRegistryWithCache(logger, client, cache, []RepoSource{source})

	// Find existing template
	tmpl, err := registry.GetTemplate("kv-store-go", false)
	require.NoError(t, err)
	assert.Equal(t, "Key-Value Store (Go)", tmpl.Title)
	assert.Equal(t, "go", tmpl.Language)

	// Template not found
	_, err = registry.GetTemplate("nonexistent", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistryMultipleSources(t *testing.T) {
	logger := testutil.NewTestLogger()
	cacheDir := t.TempDir()
	cache := NewCacheWithDir(logger, cacheDir)

	source1 := RepoSource{Owner: "org1", Repo: "templates", Ref: "main"}
	source2 := RepoSource{Owner: "org2", Repo: "custom-templates", Ref: "main"}

	// Pre-populate cache for both sources
	err := cache.SaveTemplateList(source1, []TemplateSummary{
		{
			TemplateMetadata: TemplateMetadata{
				Name:     "template-a",
				Language: "go",
				Kind:     "building-block",
			},
			Source: source1,
		},
	}, "sha1")
	require.NoError(t, err)

	err = cache.SaveTemplateList(source2, []TemplateSummary{
		{
			TemplateMetadata: TemplateMetadata{
				Name:     "template-b",
				Language: "typescript",
				Kind:     "starter-template",
			},
			Source: source2,
		},
	}, "sha2")
	require.NoError(t, err)

	client := NewClient(logger)
	registry := NewRegistryWithCache(logger, client, cache, []RepoSource{source1, source2})

	templates, err := registry.ListTemplates(false)
	require.NoError(t, err)
	assert.Len(t, templates, 2)

	// Should find templates from both sources
	tmplA, err := registry.GetTemplate("template-a", false)
	require.NoError(t, err)
	assert.Equal(t, "org1", tmplA.Source.Owner)

	tmplB, err := registry.GetTemplate("template-b", false)
	require.NoError(t, err)
	assert.Equal(t, "org2", tmplB.Source.Owner)
}

func TestRepoSourceString(t *testing.T) {
	source := RepoSource{Owner: "smartcontractkit", Repo: "cre-templates", Ref: "main"}
	assert.Equal(t, "smartcontractkit/cre-templates@main", source.String())
}

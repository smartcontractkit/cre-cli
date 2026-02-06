package templaterepo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// Registry aggregates templates from multiple repos and provides lookup/scaffolding.
type Registry struct {
	logger  *zerolog.Logger
	client  *Client
	cache   *Cache
	sources []RepoSource
}

// NewRegistry creates a new Registry with the given sources.
func NewRegistry(logger *zerolog.Logger, sources []RepoSource) (*Registry, error) {
	cache, err := NewCache(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	return &Registry{
		logger:  logger,
		client:  NewClient(logger),
		cache:   cache,
		sources: sources,
	}, nil
}

// NewRegistryWithCache creates a Registry with an injected cache (for testing).
func NewRegistryWithCache(logger *zerolog.Logger, client *Client, cache *Cache, sources []RepoSource) *Registry {
	return &Registry{
		logger:  logger,
		client:  client,
		cache:   cache,
		sources: sources,
	}
}

// ListTemplates discovers and returns all templates from configured sources.
// If refresh is true, the cache is bypassed.
func (r *Registry) ListTemplates(refresh bool) ([]TemplateSummary, error) {
	var allTemplates []TemplateSummary

	for _, source := range r.sources {
		templates, err := r.listFromSource(source, refresh)
		if err != nil {
			r.logger.Warn().Err(err).Msgf("Failed to list templates from %s", source)
			continue
		}
		allTemplates = append(allTemplates, templates...)
	}

	if len(allTemplates) == 0 {
		return nil, fmt.Errorf("no templates found from any source. Check your network connection and try again")
	}

	return allTemplates, nil
}

// GetTemplate looks up a template by name from all sources.
func (r *Registry) GetTemplate(name string, refresh bool) (*TemplateSummary, error) {
	templates, err := r.ListTemplates(refresh)
	if err != nil {
		return nil, err
	}

	for i := range templates {
		if templates[i].Name == name {
			return &templates[i], nil
		}
	}

	return nil, fmt.Errorf("template %q not found", name)
}

// ScaffoldTemplate downloads and extracts a template into destDir,
// then renames the template's workflow directory to the user's workflow name.
func (r *Registry) ScaffoldTemplate(tmpl *TemplateSummary, destDir, workflowName string, onProgress func(string)) error {
	if onProgress != nil {
		onProgress("Downloading template...")
	}

	// Try to use cached tarball
	treeSHA := r.getTreeSHA(tmpl.Source)
	if treeSHA != "" && r.cache.IsTarballCached(tmpl.Source, treeSHA) {
		r.logger.Debug().Msg("Using cached tarball")
		tarballPath := r.cache.TarballPath(tmpl.Source, treeSHA)
		err := r.client.DownloadAndExtractTemplateFromCache(tarballPath, tmpl.Path, destDir, tmpl.Exclude)
		if err == nil {
			return r.renameWorkflowDir(tmpl, destDir, workflowName)
		}
		r.logger.Warn().Err(err).Msg("Failed to extract from cached tarball, re-downloading")
	}

	// Download and cache tarball
	if treeSHA == "" {
		treeSHA = "latest"
	}
	tarballPath := r.cache.TarballPath(tmpl.Source, treeSHA)
	if err := r.client.DownloadTarball(tmpl.Source, tarballPath); err != nil {
		// Fall back to streaming download without caching
		r.logger.Debug().Msg("Falling back to streaming download")
		err = r.client.DownloadAndExtractTemplate(tmpl.Source, tmpl.Path, destDir, tmpl.Exclude, onProgress)
		if err != nil {
			return fmt.Errorf("failed to download template: %w", err)
		}
		return r.renameWorkflowDir(tmpl, destDir, workflowName)
	}

	if onProgress != nil {
		onProgress("Extracting template files...")
	}

	err := r.client.DownloadAndExtractTemplateFromCache(tarballPath, tmpl.Path, destDir, tmpl.Exclude)
	if err != nil {
		return fmt.Errorf("failed to extract template: %w", err)
	}

	return r.renameWorkflowDir(tmpl, destDir, workflowName)
}

// renameWorkflowDir finds a workflow-like directory in the extracted template
// and renames it to the user's workflow name.
func (r *Registry) renameWorkflowDir(tmpl *TemplateSummary, destDir, workflowName string) error {
	// Look for a directory that contains workflow source files (main.go, main.ts, workflow.yaml)
	// In the cre-templates repo, templates have a subdirectory like "my-workflow/"
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return nil // No renaming needed if we can't read the dir
	}

	// Find candidate workflow directory - look for a directory containing workflow files
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(destDir, entry.Name())

		// Check if this dir has workflow-like files
		if hasWorkflowFiles(dirPath) {
			if entry.Name() == workflowName {
				return nil // Already correctly named
			}
			targetPath := filepath.Join(destDir, workflowName)
			r.logger.Debug().Msgf("Renaming workflow dir %s -> %s", entry.Name(), workflowName)
			return os.Rename(dirPath, targetPath)
		}
	}

	// If no workflow subdirectory found, the template files are in the root.
	// Move everything into a workflow subdirectory.
	workflowDir := filepath.Join(destDir, workflowName)
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		return fmt.Errorf("failed to create workflow directory: %w", err)
	}

	for _, entry := range entries {
		if entry.Name() == workflowName {
			continue // Skip the directory we just created
		}
		src := filepath.Join(destDir, entry.Name())
		dst := filepath.Join(workflowDir, entry.Name())

		// Skip project-level files that should stay at root
		if isProjectLevelFile(entry.Name()) {
			continue
		}

		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("failed to move %s to workflow dir: %w", entry.Name(), err)
		}
	}

	return nil
}

// hasWorkflowFiles checks if a directory contains typical workflow source files.
func hasWorkflowFiles(dir string) bool {
	markers := []string{"main.go", "main.ts", "workflow.yaml"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}

// isProjectLevelFile returns true for files that should stay at the project root.
func isProjectLevelFile(name string) bool {
	projectFiles := map[string]bool{
		"project.yaml": true,
		"secrets.yaml": true,
		"go.mod":       true,
		"go.sum":       true,
		".env":         true,
		".gitignore":   true,
		"contracts":    true,
	}
	return projectFiles[name]
}

func (r *Registry) listFromSource(source RepoSource, refresh bool) ([]TemplateSummary, error) {
	// Check cache first (unless refresh is forced)
	if !refresh {
		templates, fresh := r.cache.LoadTemplateList(source)
		if fresh && templates != nil {
			return templates, nil
		}
	}

	// Discover from GitHub
	result, err := r.client.DiscoverTemplatesWithSHA(source)
	if err != nil {
		// Try stale cache as fallback
		if stale := r.cache.LoadStaleTemplateList(source); stale != nil {
			r.logger.Warn().Msg("Using stale cached template list (network unavailable)")
			return stale, nil
		}
		return nil, err
	}

	// Save to cache
	if saveErr := r.cache.SaveTemplateList(source, result.Templates, result.TreeSHA); saveErr != nil {
		r.logger.Warn().Err(saveErr).Msg("Failed to save template list to cache")
	}

	return result.Templates, nil
}

func (r *Registry) getTreeSHA(source RepoSource) string {
	path := r.cache.templateListPath(source)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cache templateListCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return ""
	}
	return cache.TreeSHA
}

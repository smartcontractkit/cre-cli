package templaterepo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

const (
	templateListCacheDuration = 1 * time.Hour
	tarballCacheDuration      = 24 * time.Hour
	cacheDirName              = "template-cache"
	creDirName                = ".cre"
)

// Cache manages template list and tarball caching at ~/.cre/template-cache/.
type Cache struct {
	logger   *zerolog.Logger
	cacheDir string
}

// templateListCache is the serialized form of a cached template list for a repo.
type templateListCache struct {
	Templates []TemplateSummary `json:"templates"`
	TreeSHA   string            `json:"tree_sha"`
	LastCheck time.Time         `json:"last_check"`
}

// NewCache creates a new Cache instance.
func NewCache(logger *zerolog.Logger) (*Cache, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, creDirName, cacheDirName)
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{
		logger:   logger,
		cacheDir: cacheDir,
	}, nil
}

// NewCacheWithDir creates a Cache with a specific directory (for testing).
func NewCacheWithDir(logger *zerolog.Logger, cacheDir string) *Cache {
	return &Cache{
		logger:   logger,
		cacheDir: cacheDir,
	}
}

// LoadTemplateList loads the cached template list for a repo. Returns nil if cache is missing or stale.
func (c *Cache) LoadTemplateList(source RepoSource) ([]TemplateSummary, bool) {
	path := c.templateListPath(source)
	data, err := os.ReadFile(path)
	if err != nil {
		c.logger.Debug().Msgf("No template list cache for %s", source)
		return nil, false
	}

	var cache templateListCache
	if err := json.Unmarshal(data, &cache); err != nil {
		c.logger.Debug().Msgf("Corrupt cache for %s, ignoring", source)
		return nil, false
	}

	if time.Since(cache.LastCheck) > templateListCacheDuration {
		c.logger.Debug().Msgf("Template list cache expired for %s", source)
		return cache.Templates, false // Return stale data but indicate it's stale
	}

	c.logger.Debug().Msgf("Using cached template list for %s (%d templates)", source, len(cache.Templates))
	return cache.Templates, true
}

// LoadStaleTemplateList loads templates even if stale (for offline fallback).
func (c *Cache) LoadStaleTemplateList(source RepoSource) []TemplateSummary {
	path := c.templateListPath(source)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var cache templateListCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}

	return cache.Templates
}

// SaveTemplateList saves the template list to cache.
func (c *Cache) SaveTemplateList(source RepoSource, templates []TemplateSummary, treeSHA string) error {
	cache := templateListCache{
		Templates: templates,
		TreeSHA:   treeSHA,
		LastCheck: time.Now(),
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	path := c.templateListPath(source)
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write cache: %w", err)
	}

	c.logger.Debug().Msgf("Saved template list cache for %s", source)
	return nil
}

// TarballPath returns the path where a tarball should be cached.
func (c *Cache) TarballPath(source RepoSource, sha string) string {
	return filepath.Join(c.cacheDir, "tarballs", fmt.Sprintf("%s-%s-%s.tar.gz", source.Owner, source.Repo, sha))
}

// IsTarballCached checks if a tarball is cached and not expired.
func (c *Cache) IsTarballCached(source RepoSource, sha string) bool {
	path := c.TarballPath(source, sha)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < tarballCacheDuration
}

// InvalidateTemplateList removes the cached template list for a repo source,
// forcing a fresh fetch on the next ListTemplates call.
func (c *Cache) InvalidateTemplateList(source RepoSource) {
	path := c.templateListPath(source)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		c.logger.Warn().Err(err).Msgf("Failed to invalidate cache for %s", source)
	} else {
		c.logger.Debug().Msgf("Invalidated template list cache for %s", source)
	}
}

func (c *Cache) templateListPath(source RepoSource) string {
	return filepath.Join(c.cacheDir, fmt.Sprintf("%s-%s-%s-templates.json", source.Owner, source.Repo, source.Ref))
}

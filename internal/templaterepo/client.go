package templaterepo

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

const (
	apiTimeout     = 6 * time.Second
	tarballTimeout = 30 * time.Second

	// templateMetadataFile is the conventional path to a template's metadata file
	// within its directory (e.g., "my-template/.cre/template.yaml").
	templateMetadataFile = ".cre/template.yaml"
)

// standardIgnores are files/dirs always excluded when extracting templates.
var standardIgnores = []string{
	".git",
	".cre",
	"node_modules",
	"bun.lock",
	"tmp",
	".DS_Store",
}

// Client handles GitHub API interactions for template discovery and download.
type Client struct {
	logger     *zerolog.Logger
	httpClient *http.Client
}

// NewClient creates a new GitHub template client.
func NewClient(logger *zerolog.Logger) *Client {
	return &Client{
		logger: logger,
		httpClient: &http.Client{
			Timeout: apiTimeout,
		},
	}
}

// treeResponse represents the GitHub Git Trees API response.
type treeResponse struct {
	SHA       string      `json:"sha"`
	Tree      []treeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

// treeEntry represents a single entry in the Git tree.
type treeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" or "tree"
}

// DiscoverTemplates uses the GitHub Tree API to find all template.yaml files,
// then fetches and parses each one to build the template list.
func (c *Client) DiscoverTemplates(source RepoSource) ([]TemplateSummary, error) {
	c.logger.Debug().Msgf("Discovering templates from %s", source)

	// Step 1: Get the full tree
	treeURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
		source.Owner, source.Repo, source.Ref)

	tree, err := c.fetchTree(treeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repo tree: %w", err)
	}

	// Step 2: Filter for .cre/template.yaml paths
	var templatePaths []string
	for _, entry := range tree.Tree {
		if entry.Type == "blob" && strings.HasSuffix(entry.Path, templateMetadataFile) {
			templatePaths = append(templatePaths, entry.Path)
		}
	}

	c.logger.Debug().Msgf("Found %d template.yaml files in %s", len(templatePaths), source)

	// Step 3: Fetch and parse each template.yaml via raw.githubusercontent.com
	var templates []TemplateSummary
	for _, path := range templatePaths {
		meta, err := c.fetchTemplateMetadata(source, path)
		if err != nil {
			c.logger.Warn().Err(err).Msgf("Skipping template at %s: failed to parse", path)
			continue
		}

		// Derive the template directory path (grandparent of .cre/template.yaml)
		templateDir := filepath.Dir(filepath.Dir(path))
		if templateDir == "." {
			templateDir = ""
		}

		templates = append(templates, TemplateSummary{
			TemplateMetadata: *meta,
			Path:             templateDir,
			Source:           source,
		})
	}

	return templates, nil
}

// DiscoverTemplatesResult holds the result along with the tree SHA for caching.
type DiscoverTemplatesResult struct {
	Templates []TemplateSummary
	TreeSHA   string
}

// DiscoverTemplatesWithSHA is like DiscoverTemplates but also returns the tree SHA.
func (c *Client) DiscoverTemplatesWithSHA(source RepoSource) (*DiscoverTemplatesResult, error) {
	c.logger.Debug().Msgf("Discovering templates from %s", source)

	treeURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
		source.Owner, source.Repo, source.Ref)

	tree, err := c.fetchTree(treeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repo tree: %w", err)
	}

	var templatePaths []string
	for _, entry := range tree.Tree {
		if entry.Type == "blob" && strings.HasSuffix(entry.Path, templateMetadataFile) {
			templatePaths = append(templatePaths, entry.Path)
		}
	}

	c.logger.Debug().Msgf("Found %d template.yaml files in %s", len(templatePaths), source)

	var templates []TemplateSummary
	for _, path := range templatePaths {
		meta, err := c.fetchTemplateMetadata(source, path)
		if err != nil {
			c.logger.Warn().Err(err).Msgf("Skipping template at %s: failed to parse", path)
			continue
		}

		templateDir := filepath.Dir(filepath.Dir(path))
		if templateDir == "." {
			templateDir = ""
		}

		templates = append(templates, TemplateSummary{
			TemplateMetadata: *meta,
			Path:             templateDir,
			Source:           source,
		})
	}

	return &DiscoverTemplatesResult{
		Templates: templates,
		TreeSHA:   tree.SHA,
	}, nil
}

// DownloadAndExtractTemplate downloads the repo tarball and extracts only files
// under the given templatePath, applying exclude patterns.
func (c *Client) DownloadAndExtractTemplate(source RepoSource, templatePath, destDir string, exclude []string, onProgress func(string)) error {
	tarballURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s",
		source.Owner, source.Repo, source.Ref)

	c.logger.Debug().Msgf("Downloading tarball from %s", tarballURL)

	if onProgress != nil {
		onProgress("Downloading template...")
	}

	client := &http.Client{Timeout: tarballTimeout}
	req, err := http.NewRequest("GET", tarballURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuthHeaders(req)
	req.Header.Set("User-Agent", "cre-cli")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download tarball: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tarball download failed with status: %s", resp.Status)
	}

	if onProgress != nil {
		onProgress("Extracting template files...")
	}

	return c.extractTarball(resp.Body, templatePath, destDir, exclude)
}

// DownloadAndExtractTemplateFromCache extracts from a cached tarball file.
func (c *Client) DownloadAndExtractTemplateFromCache(tarballPath, templatePath, destDir string, exclude []string) error {
	f, err := os.Open(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to open cached tarball: %w", err)
	}
	defer f.Close()
	return c.extractTarball(f, templatePath, destDir, exclude)
}

// DownloadTarball downloads the repo tarball to a local file and returns the path.
func (c *Client) DownloadTarball(source RepoSource, destPath string) error {
	tarballURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s",
		source.Owner, source.Repo, source.Ref)

	client := &http.Client{Timeout: tarballTimeout}
	req, err := http.NewRequest("GET", tarballURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setAuthHeaders(req)
	req.Header.Set("User-Agent", "cre-cli")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download tarball: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tarball download failed with status: %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
		return fmt.Errorf("failed to create directory for tarball: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create tarball file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("failed to write tarball: %w", err)
	}

	return nil
}

func (c *Client) fetchTree(url string) (*treeResponse, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)
	req.Header.Set("User-Agent", "cre-cli")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %s", resp.Status)
	}

	var tree treeResponse
	if err := json.NewDecoder(resp.Body).Decode(&tree); err != nil {
		return nil, fmt.Errorf("failed to decode tree response: %w", err)
	}

	return &tree, nil
}

func (c *Client) fetchTemplateMetadata(source RepoSource, path string) (*TemplateMetadata, error) {
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		source.Owner, source.Repo, source.Ref, path)

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "cre-cli")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("raw content fetch returned status %s for %s", resp.Status, path)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var meta TemplateMetadata
	if err := yaml.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse template.yaml at %s: %w", path, err)
	}

	// Support both "id" (new) and "name" (legacy) fields
	if meta.ID != "" {
		meta.Name = meta.ID
	}
	if meta.Name == "" {
		return nil, fmt.Errorf("template.yaml at %s missing required field 'name' or 'id'", path)
	}

	return &meta, nil
}

// extractTarball reads a gzip+tar stream and extracts files under templatePath to destDir.
func (c *Client) extractTarball(r io.Reader, templatePath, destDir string, exclude []string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	// GitHub tarballs have a top-level directory like "owner-repo-sha/"
	// We need to detect it and strip it.
	var topLevelPrefix string

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		// Skip PAX global/extended headers — these are metadata records, not real files
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			continue
		}

		// Detect top-level prefix from the first real directory entry
		if topLevelPrefix == "" {
			parts := strings.SplitN(header.Name, "/", 2)
			if len(parts) >= 1 {
				topLevelPrefix = parts[0] + "/"
			}
		}

		// Strip the top-level prefix
		name := strings.TrimPrefix(header.Name, topLevelPrefix)
		if name == "" {
			continue
		}

		// Check if this file is under our template path
		// When templatePath is empty, the entire repo is the template (root-level .cre/template.yaml)
		if templatePath != "" {
			if !strings.HasPrefix(name, templatePath+"/") && name != templatePath {
				continue
			}
		}

		// Get the relative path within the template
		var relPath string
		if templatePath == "" {
			relPath = name
		} else {
			relPath = strings.TrimPrefix(name, templatePath+"/")
		}
		if relPath == "" {
			continue
		}

		// Check standard ignores
		if shouldIgnore(relPath, standardIgnores) {
			continue
		}

		// Check template-specific excludes
		if shouldIgnore(relPath, exclude) {
			continue
		}

		targetPath := filepath.Join(destDir, relPath)

		// Prevent Zip Slip: ensure the target path stays within destDir
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(targetPath)+string(os.PathSeparator), cleanDest) && filepath.Clean(targetPath) != filepath.Clean(destDir) {
			return fmt.Errorf("illegal file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			c.logger.Debug().Msgf("Extracting dir: %s -> %s", name, targetPath)
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		case tar.TypeReg:
			c.logger.Debug().Msgf("Extracting file: %s -> %s", name, targetPath)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode)&0755|0600)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			f.Close()
		}
	}

	return nil
}

func (c *Client) setAuthHeaders(req *http.Request) {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// shouldIgnore checks if a relative path matches any of the ignore patterns.
func shouldIgnore(relPath string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		// Check exact match on first path component
		firstComponent := strings.SplitN(relPath, "/", 2)[0]
		if firstComponent == pattern {
			return true
		}
		// Check suffix match (e.g., "*.test.js")
		if strings.HasPrefix(pattern, "*") {
			suffix := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(relPath, suffix) {
				return true
			}
		}
		// Check prefix match for directory patterns (e.g., "tmp/")
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(relPath, pattern) || strings.HasPrefix(relPath, strings.TrimSuffix(pattern, "/")) {
				return true
			}
		}
	}
	return false
}

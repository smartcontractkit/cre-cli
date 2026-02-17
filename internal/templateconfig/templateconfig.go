package templateconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/smartcontractkit/cre-cli/internal/templaterepo"
)

const (
	configDirName  = ".cre"
	configFileName = "template.yaml"
)

// DefaultSource is the default template repository.
var DefaultSource = templaterepo.RepoSource{
	Owner: "smartcontractkit",
	Repo:  "cre-templates",
	Ref:   "feature/template-standard",
}

// Config represents the CLI template configuration file at ~/.cre/template.yaml.
type Config struct {
	TemplateRepositories []TemplateRepo `yaml:"templateRepositories"`
}

// TemplateRepo represents a template repository configuration.
type TemplateRepo struct {
	Owner string `yaml:"owner"`
	Repo  string `yaml:"repo"`
	Ref   string `yaml:"ref"`
}

// LoadTemplateSources returns the list of template sources from ~/.cre/template.yaml,
// falling back to the default source if the file doesn't exist.
func LoadTemplateSources(logger *zerolog.Logger) []templaterepo.RepoSource {
	cfg, err := loadConfigFile(logger)
	if err == nil && len(cfg.TemplateRepositories) > 0 {
		var sources []templaterepo.RepoSource
		for _, r := range cfg.TemplateRepositories {
			sources = append(sources, templaterepo.RepoSource{
				Owner: r.Owner,
				Repo:  r.Repo,
				Ref:   r.Ref,
			})
		}
		return sources
	}

	return []templaterepo.RepoSource{DefaultSource}
}

// SaveTemplateSources writes the given sources to ~/.cre/template.yaml.
func SaveTemplateSources(sources []templaterepo.RepoSource) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	dir := filepath.Join(homeDir, configDirName)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	var repos []TemplateRepo
	for _, s := range sources {
		repos = append(repos, TemplateRepo{
			Owner: s.Owner,
			Repo:  s.Repo,
			Ref:   s.Ref,
		})
	}

	cfg := Config{TemplateRepositories: repos}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	configPath := filepath.Join(dir, configFileName)
	tmp := configPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmp, configPath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// EnsureDefaultConfig creates ~/.cre/template.yaml with the default source
// if the file does not already exist.
func EnsureDefaultConfig(logger *zerolog.Logger) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, configDirName, configFileName)
	if _, err := os.Stat(configPath); err == nil {
		return nil // file already exists
	}

	logger.Debug().Msg("Creating default template config at " + configPath)
	return SaveTemplateSources([]templaterepo.RepoSource{DefaultSource})
}

// ParseRepoString parses "owner/repo@ref" into a RepoSource.
func ParseRepoString(s string) (templaterepo.RepoSource, error) {
	// Split by @
	ref := "main"
	repoPath := s
	if idx := strings.LastIndex(s, "@"); idx != -1 {
		repoPath = s[:idx]
		ref = s[idx+1:]
	}

	// Split by /
	parts := strings.SplitN(repoPath, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return templaterepo.RepoSource{}, fmt.Errorf("expected format: owner/repo[@ref], got %q", s)
	}

	return templaterepo.RepoSource{
		Owner: parts[0],
		Repo:  parts[1],
		Ref:   ref,
	}, nil
}

func loadConfigFile(logger *zerolog.Logger) (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, configDirName, configFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Debug().Msg("No template config found at " + configPath)
			return nil, err
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse template config: %w", err)
	}

	return &cfg, nil
}

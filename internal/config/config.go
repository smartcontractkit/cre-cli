package config

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
	configFileName = "config.yaml"
	envVarName     = "CRE_TEMPLATE_REPOS"
)

// DefaultSource is the default template repository.
var DefaultSource = templaterepo.RepoSource{
	Owner: "smartcontractkit",
	Repo:  "cre-templates",
	Ref:   "feature/template-standard",
}

// Config represents the CLI configuration file at ~/.cre/config.yaml.
type Config struct {
	TemplateRepositories []TemplateRepo `yaml:"templateRepositories"`
}

// TemplateRepo represents a template repository configuration.
type TemplateRepo struct {
	Owner string `yaml:"owner"`
	Repo  string `yaml:"repo"`
	Ref   string `yaml:"ref"`
}

// LoadTemplateSources returns the list of template sources, checking (in priority order):
// 1. CLI flag --template-repo (if provided)
// 2. CRE_TEMPLATE_REPOS environment variable
// 3. ~/.cre/config.yaml
// 4. Default: smartcontractkit/cre-templates@main
func LoadTemplateSources(logger *zerolog.Logger, flagRepo string) []templaterepo.RepoSource {
	// Priority 1: CLI flag
	if flagRepo != "" {
		source, err := ParseRepoString(flagRepo)
		if err != nil {
			logger.Warn().Err(err).Msgf("Invalid --template-repo value: %s, using default", flagRepo)
		} else {
			return []templaterepo.RepoSource{source}
		}
	}

	// Priority 2: Environment variable
	if envVal := os.Getenv(envVarName); envVal != "" {
		sources, err := parseEnvRepos(envVal)
		if err != nil {
			logger.Warn().Err(err).Msg("Invalid CRE_TEMPLATE_REPOS, using default")
		} else {
			return sources
		}
	}

	// Priority 3: Config file
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

	// Priority 4: Default
	return []templaterepo.RepoSource{DefaultSource}
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

func parseEnvRepos(envVal string) ([]templaterepo.RepoSource, error) {
	parts := strings.Split(envVal, ",")
	var sources []templaterepo.RepoSource
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		source, err := ParseRepoString(part)
		if err != nil {
			return nil, fmt.Errorf("invalid repo %q: %w", part, err)
		}
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("no valid repos found in CRE_TEMPLATE_REPOS")
	}
	return sources, nil
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
			logger.Debug().Msg("No config file found at " + configPath)
			return nil, err
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

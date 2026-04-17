package tenantctx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

// ContextFile is the filename for the local registry manifest.
const ContextFile = "context.yaml"

// Registry represents a single available workflow registry.
type Registry struct {
	ID               string   `yaml:"id" json:"id"`
	Label            string   `yaml:"label" json:"label"`
	Type             string   `yaml:"type" json:"type"`
	ChainSelector    *string  `yaml:"chain_selector,omitempty" json:"chainSelector,omitempty"`
	Address          *string  `yaml:"address,omitempty" json:"address,omitempty"`
	SecretsAuthFlows []string `yaml:"secrets_auth_flows" json:"secretsAuthFlows"`
}

// EnvironmentContext holds user context for a single CLI environment.
type EnvironmentContext struct {
	TenantID         string      `yaml:"tenant_id"`
	DefaultDonFamily string      `yaml:"default_don_family"`
	VaultGatewayURL  string      `yaml:"vault_gateway_url"`
	Registries       []*Registry `yaml:"registries"`
}

type getTenantConfigResponse struct {
	GetTenantConfig struct {
		TenantID         string `json:"tenantId"`
		DefaultDonFamily string `json:"defaultDonFamily"`
		VaultGatewayURL  string `json:"vaultGatewayUrl"`
		Registries       []struct {
			ID               string   `json:"id"`
			Label            string   `json:"label"`
			Type             string   `json:"type"`
			ChainSelector    *string  `json:"chainSelector"`
			Address          *string  `json:"address"`
			SecretsAuthFlows []string `json:"secretsAuthFlows"`
		} `json:"registries"`
	} `json:"getTenantConfig"`
}

const getTenantConfigQuery = `query GetTenantConfig {
  getTenantConfig {
    tenantId
    defaultDonFamily
    vaultGatewayUrl
    registries {
      id
      label
      type
      chainSelector
      address
      secretsAuthFlows
    }
  }
}`

// FetchAndWriteContext fetches the user context from the service
// and writes the registry manifest to ~/.cre/<ContextFile>.
func FetchAndWriteContext(ctx context.Context, gqlClient *graphqlclient.Client, envName string, log *zerolog.Logger) error {
	req := graphql.NewRequest(getTenantConfigQuery)

	var resp getTenantConfigResponse
	if err := gqlClient.Execute(ctx, req, &resp); err != nil {
		return fmt.Errorf("fetch user context: %w", err)
	}

	tc := resp.GetTenantConfig

	registries := make([]*Registry, 0, len(tc.Registries))
	for _, r := range tc.Registries {
		regType := mapRegistryType(r.Type)
		id := r.ID
		label := r.Label

		if regType == "on-chain" {
			id = "onchain:" + r.ID
			if r.Address != nil {
				label = fmt.Sprintf("%s (%s)", r.ID, abbreviateAddress(*r.Address))
			}
		}

		registries = append(registries, &Registry{
			ID:               id,
			Label:            label,
			Type:             regType,
			ChainSelector:    r.ChainSelector,
			Address:          r.Address,
			SecretsAuthFlows: mapSecretsAuthFlows(r.SecretsAuthFlows, log),
		})
	}

	envCtx := &EnvironmentContext{
		TenantID:         tc.TenantID,
		DefaultDonFamily: tc.DefaultDonFamily,
		VaultGatewayURL:  tc.VaultGatewayURL,
		Registries:       registries,
	}

	contextMap := map[string]*EnvironmentContext{
		strings.ToUpper(envName): envCtx,
	}

	return writeContextFile(contextMap, log)
}

func mapRegistryType(gqlType string) string {
	switch gqlType {
	case "ON_CHAIN":
		return "on-chain"
	case "OFF_CHAIN":
		return "off-chain"
	default:
		return strings.ToLower(gqlType)
	}
}

func mapSecretsAuthFlows(gqlFlows []string, log *zerolog.Logger) []string {
	flows := make([]string, 0, len(gqlFlows))
	for _, f := range gqlFlows {
		switch f {
		case "BROWSER":
			flows = append(flows, "browser")
		case "OWNER_KEY_SIGNING":
			flows = append(flows, "owner-key-signing")
		default:
			log.Debug().Str("flow", f).Msg("unknown secrets auth flow, skipping")
		}
	}
	return flows
}

func abbreviateAddress(addr string) string {
	if len(addr) <= 10 {
		return addr
	}
	return addr[:6] + "..." + addr[len(addr)-4:]
}

// LoadContext reads the registry manifest from ~/.cre/<ContextFile>
// and returns the EnvironmentContext for the given environment name.
func LoadContext(envName string) (*EnvironmentContext, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	return LoadContextFromPath(filepath.Join(home, credentials.ConfigDir, ContextFile), envName)
}

// LoadContextFromPath reads the registry manifest at the given path
// and returns the EnvironmentContext for the given environment name.
func LoadContextFromPath(path string, envName string) (*EnvironmentContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ContextFile, err)
	}

	var contextMap map[string]*EnvironmentContext
	if err := yaml.Unmarshal(data, &contextMap); err != nil {
		return nil, fmt.Errorf("parse %s: %w", ContextFile, err)
	}

	envCtx, ok := contextMap[strings.ToUpper(envName)]
	if !ok {
		return nil, fmt.Errorf("no context found for environment %q in %s", envName, ContextFile)
	}
	return envCtx, nil
}

func contextFileHasEnv(envName string) bool {
	_, err := LoadContext(envName)
	return err == nil
}

// EnsureContext guarantees the registry manifest exists for the current environment.
// API key users always fetch fresh; bearer token users use the cached file from login.
func EnsureContext(ctx context.Context, creds *credentials.Credentials, envSet *environments.EnvironmentSet, log *zerolog.Logger) error {
	envName := envSet.EnvName
	if envName == "" {
		envName = environments.DefaultEnv
	}

	alwaysFetch := creds.AuthType == credentials.AuthTypeApiKey

	if !alwaysFetch && contextFileHasEnv(envName) {
		return nil
	}

	log.Debug().Str("env", envName).Bool("api_key", alwaysFetch).Msg("fetching user context")
	gqlClient := graphqlclient.New(creds, envSet, log)
	return FetchAndWriteContext(ctx, gqlClient, envName, log)
}

func writeContextFile(data map[string]*EnvironmentContext, log *zerolog.Logger) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	dir := filepath.Join(home, credentials.ConfigDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal context: %w", err)
	}

	path := filepath.Join(dir, ContextFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	log.Debug().Str("path", path).Msg("wrote " + ContextFile)
	return nil
}

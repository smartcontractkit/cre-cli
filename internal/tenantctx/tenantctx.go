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
)

const (
	ContextFile = "context.yaml"
)

// Registry represents a single workflow registry from the GQL response.
type Registry struct {
	ID               string   `yaml:"id" json:"id"`
	Label            string   `yaml:"label" json:"label"`
	Type             string   `yaml:"type" json:"type"`
	ChainSelector    *string  `yaml:"chain_selector,omitempty" json:"chainSelector,omitempty"`
	Address          *string  `yaml:"address,omitempty" json:"address,omitempty"`
	SecretsAuthFlows []string `yaml:"secrets_auth_flows" json:"secretsAuthFlows"`
	Active           bool     `yaml:"active" json:"-"`
}

// EnvironmentContext represents the tenant config for a single environment block in context.yaml.
type EnvironmentContext struct {
	TenantID         string      `yaml:"tenant_id"`
	DefaultDonFamily string      `yaml:"default_don_family"`
	VaultGatewayURL  string      `yaml:"vault_gateway_url"`
	Registries       []*Registry `yaml:"registries"`
}

// getTenantConfigResponse mirrors the GQL response shape.
type getTenantConfigResponse struct {
	GetTenantConfig struct {
		TenantID         int    `json:"tenantId"`
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

// FetchAndWriteContext calls getTenantConfig and writes ~/.cre/context.yaml.
// envName is the CRE_CLI_ENV value (e.g. "PRODUCTION", "STAGING").
func FetchAndWriteContext(ctx context.Context, gqlClient *graphqlclient.Client, envName string, log *zerolog.Logger) error {
	req := graphql.NewRequest(getTenantConfigQuery)

	var resp getTenantConfigResponse
	if err := gqlClient.Execute(ctx, req, &resp); err != nil {
		return fmt.Errorf("fetch tenant config: %w", err)
	}

	tc := resp.GetTenantConfig

	registries := make([]*Registry, 0, len(tc.Registries))
	for _, r := range tc.Registries {
		registries = append(registries, &Registry{
			ID:               r.ID,
			Label:            r.Label,
			Type:             mapRegistryType(r.Type),
			ChainSelector:    r.ChainSelector,
			Address:          r.Address,
			SecretsAuthFlows: r.SecretsAuthFlows,
			Active:           false,
		})
	}

	// Default the first registry to active
	if len(registries) > 0 {
		registries[0].Active = true
	}

	envCtx := &EnvironmentContext{
		TenantID:         fmt.Sprintf("%d", tc.TenantID),
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

	log.Debug().Str("path", path).Msg("wrote context.yaml")
	return nil
}

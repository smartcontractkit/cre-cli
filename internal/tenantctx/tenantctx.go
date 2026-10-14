package tenantctx

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/creconfig"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/registrytype"
)

// ContextFile is the filename for the local registry manifest.
const ContextFile = "context.yaml"

// Bounds how long server-side tenant changes stay invisible to bearer users.
const contextTTL = 24 * time.Hour

// Set by the root command; importing cmd/version here would create a cycle.
var CLIVersion = "development"

// Registry represents a single available workflow registry.
type Registry struct {
	ID            string  `yaml:"id" json:"id"`
	Label         string  `yaml:"label" json:"label"`
	Type          string  `yaml:"type" json:"type"`
	ChainSelector *string `yaml:"chain_selector,omitempty" json:"chainSelector,omitempty"`
	Address       *string `yaml:"address,omitempty" json:"address,omitempty"`
}

// Forwarder is a chain selector and mock forwarder contract address for the tenant.
type Forwarder struct {
	ChainSelector uint64 `yaml:"chain_selector" json:"chainSelector"`
	Address       string `yaml:"address" json:"address"`
}

// OnChainContract is a chain selector and contract address pair.
type OnChainContract struct {
	ChainSelector uint64 `yaml:"chain_selector" json:"chainSelector"`
	Address       string `yaml:"address" json:"address"`
}

// EnvironmentContext holds user context for a single CLI environment.
type EnvironmentContext struct {
	TenantID             string           `yaml:"tenant_id"`
	DefaultDonFamily     string           `yaml:"default_don_family"`
	VaultGatewayURL      string           `yaml:"vault_gateway_url"`
	CapabilitiesRegistry *OnChainContract `yaml:"capabilities_registry,omitempty"`
	Registries           []*Registry      `yaml:"registries"`
	Forwarders           []Forwarder      `yaml:"forwarders,omitempty"`
	FetchedAt            time.Time        `yaml:"fetched_at,omitempty"`
	CLIVersion           string           `yaml:"cli_version,omitempty"`
}

type gqlForwarder struct {
	ChainSelector json.RawMessage `json:"chainSelector"`
	Address       string          `json:"address"`
}

type gqlOnChainContract struct {
	ChainSelector json.RawMessage `json:"chainSelector"`
	Address       string          `json:"address"`
}

type getTenantConfigResponse struct {
	GetTenantConfig struct {
		TenantID             string             `json:"tenantId"`
		DefaultDonFamily     string             `json:"defaultDonFamily"`
		VaultGatewayURL      string             `json:"vaultGatewayUrl"`
		CapabilitiesRegistry gqlOnChainContract `json:"capabilitiesRegistry"`
		Registries           []struct {
			ID            string  `json:"id"`
			Label         string  `json:"label"`
			Type          string  `json:"type"`
			ChainSelector *string `json:"chainSelector"`
			Address       *string `json:"address"`
		} `json:"registries"`
		Forwarders []gqlForwarder `json:"forwarders"`
	} `json:"getTenantConfig"`
}

const getTenantConfigQuery = `query GetTenantConfig {
  getTenantConfig {
    tenantId
    defaultDonFamily
    vaultGatewayUrl
    capabilitiesRegistry {
      chainSelector
      address
    }
    registries {
      id
      label
      type
      chainSelector
      address
    }
    forwarders {
      chainSelector
      address
    }
  }
}`

// FetchAndWriteContext fetches the user context from the service
// and writes the registry manifest to the CLI config directory.
func FetchAndWriteContext(ctx context.Context, gqlClient *graphqlclient.Client, envName string, log *zerolog.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	req := graphql.NewRequest(getTenantConfigQuery)

	var resp getTenantConfigResponse
	if err := gqlClient.Execute(ctx, req, &resp); err != nil {
		return fmt.Errorf("fetch user context: %w", err)
	}

	tc := resp.GetTenantConfig

	registries := make([]*Registry, 0, len(tc.Registries))
	for _, r := range tc.Registries {
		regType := registrytype.FromGQL(r.Type, log)
		id := r.ID
		label := r.Label

		if regType == registrytype.OnChain {
			id = "onchain:" + r.ID
			if r.Address != nil {
				label = fmt.Sprintf("%s (%s)", r.ID, abbreviateAddress(*r.Address))
			}
		}

		registries = append(registries, &Registry{
			ID:            id,
			Label:         label,
			Type:          string(regType),
			ChainSelector: r.ChainSelector,
			Address:       r.Address,
		})
	}

	forwarders := make([]Forwarder, 0, len(tc.Forwarders))
	for _, f := range tc.Forwarders {
		sel, err := parseChainSelectorJSON(f.ChainSelector)
		if err != nil {
			log.Warn().Err(err).Str("address", f.Address).Msg("skipping forwarder with invalid chainSelector")
			continue
		}
		addr := strings.TrimSpace(f.Address)
		if addr == "" {
			log.Warn().Uint64("chainSelector", sel).Msg("skipping forwarder with empty address")
			continue
		}
		forwarders = append(forwarders, Forwarder{ChainSelector: sel, Address: addr})
	}

	capRegSel, err := parseChainSelectorJSON(tc.CapabilitiesRegistry.ChainSelector)
	if err != nil {
		return fmt.Errorf("invalid capabilitiesRegistry chainSelector: %w", err)
	}
	capRegAddr := strings.TrimSpace(tc.CapabilitiesRegistry.Address)
	if capRegAddr == "" {
		return fmt.Errorf("capabilitiesRegistry address is empty")
	}

	envCtx := &EnvironmentContext{
		TenantID:         tc.TenantID,
		DefaultDonFamily: tc.DefaultDonFamily,
		VaultGatewayURL:  tc.VaultGatewayURL,
		CapabilitiesRegistry: &OnChainContract{
			ChainSelector: capRegSel,
			Address:       capRegAddr,
		},
		Registries: registries,
		Forwarders: forwarders,
		FetchedAt:  time.Now().UTC(),
		CLIVersion: CLIVersion,
	}

	contextMap := map[string]*EnvironmentContext{
		strings.ToUpper(envName): envCtx,
	}

	return writeContextFile(contextMap, log)
}

func abbreviateAddress(addr string) string {
	if len(addr) <= 10 {
		return addr
	}
	return addr[:6] + "..." + addr[len(addr)-4:]
}

// parseChainSelectorJSON decodes chainSelector from GraphQL JSON (string or number).
// Prefer string values in the API response to avoid loss of precision for large selectors.
func parseChainSelectorJSON(raw []byte) (uint64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("empty chain selector")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return strconv.ParseUint(string(n), 10, 64)
	}
	return 0, fmt.Errorf("chain selector must be a decimal string or integer JSON value: %s", string(raw))
}

// LoadContext reads the registry manifest from the CLI config directory
// and returns the EnvironmentContext for the given environment name.
func LoadContext(envName string) (*EnvironmentContext, error) {
	path, err := creconfig.FilePath(ContextFile)
	if err != nil {
		return nil, err
	}
	return LoadContextFromPath(path, envName)
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

// ClearContext forces the next command to refetch the user context.
func ClearContext() error {
	path, err := creconfig.FilePath(ContextFile)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// A different CLI version may expect fields or environments the cache predates.
func isFresh(envCtx *EnvironmentContext, now time.Time) bool {
	return envCtx.CLIVersion == CLIVersion && now.Sub(envCtx.FetchedAt) < contextTTL
}

// EnsureContext guarantees a fresh registry manifest exists for the current environment.
// API key users always fetch; bearer users reuse the cache until it is stale.
func EnsureContext(ctx context.Context, creds *credentials.Credentials, envSet *environments.EnvironmentSet, log *zerolog.Logger) error {
	envName := envSet.EnvName
	if envName == "" {
		envName = environments.DefaultEnv
	}

	alwaysFetch := creds.AuthType == credentials.AuthTypeApiKey
	cached, cacheErr := LoadContext(envName)
	hasCache := !alwaysFetch && cacheErr == nil

	if hasCache && isFresh(cached, time.Now()) {
		return nil
	}

	log.Debug().Str("env", envName).Bool("api_key", alwaysFetch).Msg("fetching user context")
	gqlClient := graphqlclient.New(creds, envSet, log)
	err := FetchAndWriteContext(ctx, gqlClient, envName, log)
	if err != nil && hasCache {
		// A stale cache beats failing every command while the API is unreachable.
		log.Debug().Err(err).Msg("failed to refresh user context; using cached copy")
		return nil
	}
	return err
}

func writeContextFile(data map[string]*EnvironmentContext, log *zerolog.Logger) error {
	dir, err := creconfig.EnsureDir()
	if err != nil {
		return err
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal context: %w", err)
	}

	path := filepath.Join(dir, ContextFile)
	// Unique temp name so concurrent cre processes refreshing at once don't clobber each other.
	tmp, err := os.CreateTemp(dir, ContextFile+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	log.Debug().Str("path", path).Msg("wrote " + ContextFile)
	return nil
}

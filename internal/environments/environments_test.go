package environments

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

const testYAML = `ENVIRONMENTS:
  DEVELOPMENT:
    CRE_CLI_AUTH_BASE: https://auth0.test
    CRE_CLI_COGNITO_URL: https://cognito.test
    CRE_CLI_CLIENT_ID: test-id
    CRE_CLI_GRAPHQL_URL: https://graphql.test
    CRE_CLI_AUDIENCE: development-aud
    CRE_CLI_USER_POOL_ID: pool-id

    CRE_CLI_WORKFLOW_REGISTRY_ADDRESS: "0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D"
    CRE_CLI_WORKFLOW_REGISTRY_CHAIN_NAME: "ethereum-testnet-sepolia-base-1"

  STAGING:
    CRE_CLI_AUTH_BASE: https://staging.auth0
    CRE_CLI_COGNITO_URL: https://staging.cognito
    CRE_CLI_CLIENT_ID: staging-id
    CRE_CLI_GRAPHQL_URL: https://staging.graphql
    CRE_CLI_AUDIENCE: staging-aud
    CRE_CLI_USER_POOL_ID: staging-pool

    CRE_CLI_WORKFLOW_REGISTRY_ADDRESS: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    CRE_CLI_WORKFLOW_REGISTRY_CHAIN_NAME: "ethereum-mainnet"
`

func TestLoadEnvironmentFile(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "env.yaml")
	if err := os.WriteFile(file, []byte(testYAML), 0600); err != nil {
		t.Fatalf("failed to write temp YAML: %v", err)
	}

	ff, err := loadEnvironmentFile(file)
	if err != nil {
		t.Fatalf("LoadEnvironmentFile returned error: %v", err)
	}

	sbx, ok := ff.Envs["DEVELOPMENT"]
	if !ok {
		t.Fatal("DEVELOPMENT environment missing")
	}
	if sbx.AuthBase != "https://auth0.test" {
		t.Errorf("AuthBase = %q; want https://auth0.test", sbx.AuthBase)
	}
	if sbx.ClientID != "test-id" {
		t.Errorf("ClientID = %q; want test-id", sbx.ClientID)
	}
	if sbx.GraphQLURL != "https://graphql.test" {
		t.Errorf("GraphQLURL = %q; want https://graphql.test", sbx.GraphQLURL)
	}
	if sbx.Audience != "development-aud" {
		t.Errorf("Audience = %q; want development-aud", sbx.Audience)
	}

	if sbx.WorkflowRegistryAddress != "0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D" {
		t.Errorf("WorkflowRegistryAddress = %q; want 0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D", sbx.WorkflowRegistryAddress)
	}
	if sbx.WorkflowRegistryChainName != "ethereum-testnet-sepolia-base-1" {
		t.Errorf("WorkflowRegistryChainName = %q; want ethereum-testnet-sepolia-base-1", sbx.WorkflowRegistryChainName)
	}
}

func TestNewEnvironmentSet_FallbackAndOverrides(t *testing.T) {
	ff := &fileFormat{Envs: map[string]EnvironmentSet{
		"DEVELOPMENT": {
			AuthBase:   "b",
			ClientID:   "c",
			GraphQLURL: "d",
			Audience:   "aa",

			WorkflowRegistryAddress:   "0xdevelopment_wr",
			WorkflowRegistryChainName: "ethereum-testnet-sepolia",
		},
		"STAGING": {
			AuthBase:   "g",
			ClientID:   "h",
			GraphQLURL: "i",
			Audience:   "bb",

			WorkflowRegistryAddress:   "0xstaging_wr",
			WorkflowRegistryChainName: "polygon-mainnet",
		},
	}}

	t.Setenv(EnvVarAuthBase, "")
	t.Setenv(EnvVarClientID, "")
	t.Setenv(EnvVarGraphQLURL, "")
	t.Setenv(EnvVarAudience, "")

	set := NewEnvironmentSet(ff, "DEVELOPMENT")
	if set.AuthBase != "b" {
		t.Errorf("fallback AuthBase = %q; want b", set.AuthBase)
	}
	if set.Audience != "aa" {
		t.Errorf("fallback Audience = %q; want aa", set.Audience)
	}
	t.Setenv(EnvVarAudience, "")
	t.Setenv(EnvVarWorkflowRegistryAddress, "")
	t.Setenv(EnvVarWorkflowRegistryChainName, "")

	set2 := NewEnvironmentSet(ff, "STAGING")
	if set2.ClientID != "h" {
		t.Errorf("staging ClientID = %q; want h", set2.ClientID)
	}
	if set2.WorkflowRegistryAddress != "0xstaging_wr" {
		t.Errorf("WorkflowRegistryAddress = %q; want 0xstaging_wr", set2.WorkflowRegistryAddress)
	}
	if set2.WorkflowRegistryChainName != "polygon-mainnet" {
		t.Errorf("WorkflowRegistryChainName = %q; want polygon-mainnet", set2.WorkflowRegistryChainName)
	}

	t.Setenv(EnvVarClientID, "override-id")
	t.Setenv(EnvVarAuthBase, "override-auth")
	t.Setenv(EnvVarAudience, "override-aud")
	t.Setenv(EnvVarWorkflowRegistryAddress, "0xoverride_wr")
	t.Setenv(EnvVarWorkflowRegistryChainName, "ethereum-testnet-arbitrum-1")

	set3 := NewEnvironmentSet(ff, "STAGING")
	if set3.ClientID != "override-id" {
		t.Errorf("overridden ClientID = %q; want override-id", set3.ClientID)
	}
	if set3.AuthBase != "override-auth" {
		t.Errorf("overridden AuthBase = %q; want override-auth", set3.AuthBase)
	}
	if set3.Audience != "override-aud" {
		t.Errorf("overridden Audience = %q; want override-aud", set3.Audience)
	}
	if set3.WorkflowRegistryAddress != "0xoverride_wr" {
		t.Errorf("WorkflowRegistryAddress = %q; want 0xoverride_wr", set3.WorkflowRegistryAddress)
	}
	if set3.WorkflowRegistryChainName != "ethereum-testnet-arbitrum-1" {
		t.Errorf("WorkflowRegistryChainName = %q; want ethereum-testnet-arbitrum-1", set3.WorkflowRegistryChainName)
	}

}

func loadEnvironmentFile(path string) (*fileFormat, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading environments file from disk: %w", err)
	}
	var ff fileFormat
	if err := yaml.Unmarshal(data, &ff); err != nil {
		return nil, fmt.Errorf("unmarshalling environments file: %w", err)
	}
	return &ff, nil
}

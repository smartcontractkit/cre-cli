package environments

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

const testYAML = `ENVIRONMENTS:
  SANDBOX:
    CRE_CLI_AUTH_BASE: https://auth0.test
    CRE_CLI_COGNITO_URL: https://cognito.test
    CRE_CLI_CLIENT_ID: test-id
    CRE_CLI_GRAPHQL_URL: https://graphql.test
    CRE_CLI_AUDIENCE: sandbox-aud
    CRE_CLI_USER_POOL_ID: pool-id

    CRE_CLI_WORKFLOW_REGISTRY_ADDRESS: "0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D"
    CRE_CLI_WORKFLOW_REGISTRY_CHAIN_NAME: "ethereum-testnet-sepolia-base-1"

    CRE_CLI_CAPABILITIES_REGISTRY_ADDRESS: "0x02471a03A86ac92DE793af303E9f756f0aE60677"
    CRE_CLI_CAPABILITIES_REGISTRY_CHAIN_NAME: "ethereum-testnet-sepolia"

  STAGING:
    CRE_CLI_AUTH_BASE: https://staging.auth0
    CRE_CLI_COGNITO_URL: https://staging.cognito
    CRE_CLI_CLIENT_ID: staging-id
    CRE_CLI_GRAPHQL_URL: https://staging.graphql
    CRE_CLI_AUDIENCE: staging-aud
    CRE_CLI_USER_POOL_ID: staging-pool

    CRE_CLI_WORKFLOW_REGISTRY_ADDRESS: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    CRE_CLI_WORKFLOW_REGISTRY_CHAIN_NAME: "ethereum-mainnet"

    CRE_CLI_CAPABILITIES_REGISTRY_ADDRESS: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    CRE_CLI_CAPABILITIES_REGISTRY_CHAIN_NAME: "polygon-mainnet"
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

	sbx, ok := ff.Envs["SANDBOX"]
	if !ok {
		t.Fatal("SANDBOX environment missing")
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
	if sbx.Audience != "sandbox-aud" {
		t.Errorf("Audience = %q; want sandbox-aud", sbx.Audience)
	}

	if sbx.WorkflowRegistryAddress != "0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D" {
		t.Errorf("WorkflowRegistryAddress = %q; want 0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D", sbx.WorkflowRegistryAddress)
	}
	if sbx.WorkflowRegistryChainName != "ethereum-testnet-sepolia-base-1" {
		t.Errorf("WorkflowRegistryChainName = %q; want ethereum-testnet-sepolia-base-1", sbx.WorkflowRegistryChainName)
	}
	if sbx.CapabilitiesRegistryAddress != "0x02471a03A86ac92DE793af303E9f756f0aE60677" {
		t.Errorf("CapabilitiesRegistryAddress = %q; want 0x02471a03A86ac92DE793af303E9f756f0aE60677", sbx.CapabilitiesRegistryAddress)
	}
	if sbx.CapabilitiesRegistryChainName != "ethereum-testnet-sepolia" {
		t.Errorf("CapabilitiesRegistryChainName = %q; want ethereum-testnet-sepolia", sbx.CapabilitiesRegistryChainName)
	}
}

func TestNewEnvironmentSet_FallbackAndOverrides(t *testing.T) {
	ff := &fileFormat{Envs: map[string]EnvironmentSet{
		"SANDBOX": {
			AuthBase:   "b",
			ClientID:   "c",
			GraphQLURL: "d",
			Audience:   "aa",

			WorkflowRegistryAddress:       "0xsandbox_wr",
			WorkflowRegistryChainName:     "ethereum-testnet-sepolia",
			CapabilitiesRegistryAddress:   "0xsandbox_cap",
			CapabilitiesRegistryChainName: "ethereum-testnet-sepolia-base-1",
		},
		"STAGING": {
			AuthBase:   "g",
			ClientID:   "h",
			GraphQLURL: "i",
			Audience:   "bb",

			WorkflowRegistryAddress:       "0xstaging_wr",
			WorkflowRegistryChainName:     "polygon-mainnet",
			CapabilitiesRegistryAddress:   "0xstaging_cap",
			CapabilitiesRegistryChainName: "ethereum-mainnet",
		},
	}}

	t.Setenv(EnvVarAuthBase, "")
	t.Setenv(EnvVarClientID, "")
	t.Setenv(EnvVarGraphQLURL, "")
	t.Setenv(EnvVarAudience, "")

	set := NewEnvironmentSet(ff, "SANDBOX")
	if set.AuthBase != "b" {
		t.Errorf("fallback AuthBase = %q; want b", set.AuthBase)
	}
	if set.Audience != "aa" {
		t.Errorf("fallback Audience = %q; want aa", set.Audience)
	}
	t.Setenv(EnvVarAudience, "")
	t.Setenv(EnvVarWorkflowRegistryAddress, "")
	t.Setenv(EnvVarCapabilitiesRegistryAddress, "")
	t.Setenv(EnvVarWorkflowRegistryChainName, "")
	t.Setenv(EnvVarCapabilitiesRegistryChainName, "")

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
	if set2.CapabilitiesRegistryAddress != "0xstaging_cap" {
		t.Errorf("CapabilitiesRegistryAddress = %q; want 0xstaging_cap", set2.CapabilitiesRegistryAddress)
	}
	if set2.CapabilitiesRegistryChainName != "ethereum-mainnet" {
		t.Errorf("CapabilitiesRegistryChainName = %q; want ethereum-mainnet", set2.CapabilitiesRegistryChainName)
	}

	t.Setenv(EnvVarClientID, "override-id")
	t.Setenv(EnvVarAuthBase, "override-auth")
	t.Setenv(EnvVarAudience, "override-aud")
	t.Setenv(EnvVarWorkflowRegistryAddress, "0xoverride_wr")
	t.Setenv(EnvVarCapabilitiesRegistryAddress, "0xoverride_cap")
	t.Setenv(EnvVarWorkflowRegistryChainName, "ethereum-testnet-arbitrum-1")
	t.Setenv(EnvVarCapabilitiesRegistryChainName, "ethereum-testnet-optimism-1")

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
	if set3.CapabilitiesRegistryAddress != "0xoverride_cap" {
		t.Errorf("CapabilitiesRegistryAddress = %q; want 0xoverride_cap", set3.CapabilitiesRegistryAddress)
	}
	if set3.WorkflowRegistryChainName != "ethereum-testnet-arbitrum-1" {
		t.Errorf("WorkflowRegistryChainName = %q; want ethereum-testnet-arbitrum-1", set3.WorkflowRegistryChainName)
	}
	if set3.CapabilitiesRegistryChainName != "ethereum-testnet-optimism-1" {
		t.Errorf("CapabilitiesRegistryChainName = %q; want ethereum-testnet-optimism-1", set3.CapabilitiesRegistryChainName)
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

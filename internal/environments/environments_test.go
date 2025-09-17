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
    CRE_CLI_UI_URL: http://localhost:3000
    CRE_CLI_AUTH_BASE: https://auth0.test
    CRE_CLI_COGNITO_URL: https://cognito.test
    CRE_CLI_CLIENT_ID: test-id
    CRE_CLI_GRAPHQL_URL: https://graphql.test
    CRE_CLI_AUDIENCE: sandbox-aud
    CRE_CLI_USER_POOL_ID: pool-id

    CRE_CLI_WORKFLOW_REGISTRY_ADDRESS: "0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D"
    CRE_CLI_WORKFLOW_REGISTRY_CHAIN_SELECTOR: 10344971235874465080

    CRE_CLI_CAPABILITIES_REGISTRY_ADDRESS: "0x02471a03A86ac92DE793af303E9f756f0aE60677"
    CRE_CLI_CAPABILITIES_REGISTRY_CHAIN_SELECTOR: 16015286601757825753

  STAGING:
    CRE_CLI_UI_URL: https://staging.ui
    CRE_CLI_AUTH_BASE: https://staging.auth0
    CRE_CLI_COGNITO_URL: https://staging.cognito
    CRE_CLI_CLIENT_ID: staging-id
    CRE_CLI_GRAPHQL_URL: https://staging.graphql
    CRE_CLI_AUDIENCE: staging-aud
    CRE_CLI_USER_POOL_ID: staging-pool

    CRE_CLI_WORKFLOW_REGISTRY_ADDRESS: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    CRE_CLI_WORKFLOW_REGISTRY_CHAIN_SELECTOR: 1234567890123456789

    CRE_CLI_CAPABILITIES_REGISTRY_ADDRESS: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    CRE_CLI_CAPABILITIES_REGISTRY_CHAIN_SELECTOR: 9876543210987654321
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
	if sbx.WorkflowRegistryChainSelector != 10344971235874465080 {
		t.Errorf("WorkflowRegistryChainSelector = %d; want 10344971235874465080", sbx.WorkflowRegistryChainSelector)
	}
	if sbx.CapabilitiesRegistryAddress != "0x02471a03A86ac92DE793af303E9f756f0aE60677" {
		t.Errorf("CapabilitiesRegistryAddress = %q; want 0x02471a03A86ac92DE793af303E9f756f0aE60677", sbx.CapabilitiesRegistryAddress)
	}
	if sbx.CapabilitiesRegistryChainSelector != 16015286601757825753 {
		t.Errorf("CapabilitiesRegistryChainSelector = %d; want 16015286601757825753", sbx.CapabilitiesRegistryChainSelector)
	}
}

func TestNewEnvironmentSet_FallbackAndOverrides(t *testing.T) {
	ff := &fileFormat{Envs: map[string]EnvironmentSet{
		"SANDBOX": {
			UIURL:      "a",
			AuthBase:   "b",
			ClientID:   "c",
			GraphQLURL: "d",
			Audience:   "aa",

			WorkflowRegistryAddress:           "0xsandbox_wr",
			WorkflowRegistryChainSelector:     101,
			CapabilitiesRegistryAddress:       "0xsandbox_cap",
			CapabilitiesRegistryChainSelector: 201,
		},
		"STAGING": {
			UIURL:      "f",
			AuthBase:   "g",
			ClientID:   "h",
			GraphQLURL: "i",
			Audience:   "bb",

			WorkflowRegistryAddress:           "0xstaging_wr",
			WorkflowRegistryChainSelector:     111,
			CapabilitiesRegistryAddress:       "0xstaging_cap",
			CapabilitiesRegistryChainSelector: 211,
		},
	}}

	t.Setenv(EnvVarAuthBase, "")
	t.Setenv(EnvVarClientID, "")
	t.Setenv(EnvVarGraphQLURL, "")
	t.Setenv(EnvVarAudience, "")

	set := NewEnvironmentSet(ff, "SANDBOX")
	if set.UIURL != "a" {
		t.Errorf("fallback UIURL = %q; want a", set.UIURL)
	}
	if set.AuthBase != "b" {
		t.Errorf("fallback AuthBase = %q; want b", set.AuthBase)
	}
	if set.Audience != "aa" {
		t.Errorf("fallback Audience = %q; want aa", set.Audience)
	}
	t.Setenv(EnvVarAudience, "")
	t.Setenv(EnvVarWorkflowRegistryAddress, "")
	t.Setenv(EnvVarCapabilitiesRegistryAddress, "")
	t.Setenv(EnvVarWorkflowRegistryChainSelector, "")
	t.Setenv(EnvVarCapabilitiesRegistryChainSelector, "")

	set2 := NewEnvironmentSet(ff, "STAGING")
	if set2.ClientID != "h" {
		t.Errorf("staging ClientID = %q; want h", set2.ClientID)
	}
	if set2.WorkflowRegistryAddress != "0xstaging_wr" {
		t.Errorf("WorkflowRegistryAddress = %q; want 0xstaging_wr", set2.WorkflowRegistryAddress)
	}
	if set2.WorkflowRegistryChainSelector != 111 {
		t.Errorf("WorkflowRegistryChainSelector = %d; want 111", set2.WorkflowRegistryChainSelector)
	}
	if set2.CapabilitiesRegistryAddress != "0xstaging_cap" {
		t.Errorf("CapabilitiesRegistryAddress = %q; want 0xstaging_cap", set2.CapabilitiesRegistryAddress)
	}
	if set2.CapabilitiesRegistryChainSelector != 211 {
		t.Errorf("CapabilitiesRegistryChainSelector = %d; want 211", set2.CapabilitiesRegistryChainSelector)
	}

	t.Setenv(EnvVarClientID, "override-id")
	t.Setenv(EnvVarAuthBase, "override-auth")
	t.Setenv(EnvVarAudience, "override-aud")
	t.Setenv(EnvVarWorkflowRegistryAddress, "0xoverride_wr")
	t.Setenv(EnvVarCapabilitiesRegistryAddress, "0xoverride_cap")
	t.Setenv(EnvVarWorkflowRegistryChainSelector, "123456")
	t.Setenv(EnvVarCapabilitiesRegistryChainSelector, "654321")

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
	if set3.WorkflowRegistryChainSelector != 123456 {
		t.Errorf("WorkflowRegistryChainSelector = %d; want 123456", set3.WorkflowRegistryChainSelector)
	}
	if set3.CapabilitiesRegistryChainSelector != 654321 {
		t.Errorf("CapabilitiesRegistryChainSelector = %d; want 654321", set3.CapabilitiesRegistryChainSelector)
	}

	t.Setenv(EnvVarWorkflowRegistryChainSelector, "not-a-number")
	t.Setenv(EnvVarCapabilitiesRegistryChainSelector, "also-bad")

	set4 := NewEnvironmentSet(ff, "STAGING")
	if set4.WorkflowRegistryChainSelector != 111 {
		t.Errorf("invalid override kept? WorkflowRegistryChainSelector = %d; want 111", set4.WorkflowRegistryChainSelector)
	}
	if set4.CapabilitiesRegistryChainSelector != 211 {
		t.Errorf("invalid override kept? CapabilitiesRegistryChainSelector = %d; want 211", set4.CapabilitiesRegistryChainSelector)
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

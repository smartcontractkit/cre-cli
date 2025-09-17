package environments

import (
	"embed"
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v2"
)

const (
	EnvVarEnv = "CRE_CLI_ENV"

	EnvVarUIURL           = "CRE_CLI_UI_URL"
	EnvVarAuthBase        = "CRE_CLI_AUTH_BASE"
	EnvVarClientID        = "CRE_CLI_CLIENT_ID"
	EnvVarGraphQLURL      = "CRE_CLI_GRAPHQL_URL"
	EnvVarAudience        = "CRE_CLI_AUDIENCE"
	EnvVarVaultGatewayURL = "CRE_VAULT_DON_GATEWAY_URL"

	EnvVarWorkflowRegistryAddress           = "CRE_CLI_WORKFLOW_REGISTRY_ADDRESS"
	EnvVarWorkflowRegistryChainSelector     = "CRE_CLI_WORKFLOW_REGISTRY_CHAIN_SELECTOR"
	EnvVarCapabilitiesRegistryAddress       = "CRE_CLI_CAPABILITIES_REGISTRY_ADDRESS"
	EnvVarCapabilitiesRegistryChainSelector = "CRE_CLI_CAPABILITIES_REGISTRY_CHAIN_SELECTOR"

	DefaultEnv = "SANDBOX"
)

//go:embed environments.yaml
var envFileContent embed.FS

type EnvironmentSet struct {
	UIURL      string `yaml:"CRE_CLI_UI_URL"`
	AuthBase   string `yaml:"CRE_CLI_AUTH_BASE"`
	ClientID   string `yaml:"CRE_CLI_CLIENT_ID"`
	GraphQLURL string `yaml:"CRE_CLI_GRAPHQL_URL"`
	Audience   string `yaml:"CRE_CLI_AUDIENCE"`
	GatewayURL string `yaml:"CRE_VAULT_DON_GATEWAY_URL"`

	WorkflowRegistryAddress           string `yaml:"CRE_CLI_WORKFLOW_REGISTRY_ADDRESS"`
	WorkflowRegistryChainSelector     uint64 `yaml:"CRE_CLI_WORKFLOW_REGISTRY_CHAIN_SELECTOR"`
	CapabilitiesRegistryAddress       string `yaml:"CRE_CLI_CAPABILITIES_REGISTRY_ADDRESS"`
	CapabilitiesRegistryChainSelector uint64 `yaml:"CRE_CLI_CAPABILITIES_REGISTRY_CHAIN_SELECTOR"`
}

type fileFormat struct {
	Envs map[string]EnvironmentSet `yaml:"ENVIRONMENTS"`
}

func loadEmbeddedEnvironmentFile() (*fileFormat, error) {
	data, err := envFileContent.ReadFile("environments.yaml")
	if err != nil {
		return nil, fmt.Errorf("reading embedded environments file: %w", err)
	}
	var ff fileFormat
	if err := yaml.Unmarshal(data, &ff); err != nil {
		return nil, fmt.Errorf("unmarshalling embedded environments file: %w", err)
	}
	return &ff, nil
}

func NewEnvironmentSet(ff *fileFormat, envName string) *EnvironmentSet {
	set, ok := ff.Envs[envName]
	if !ok {
		set = ff.Envs[DefaultEnv]
	}
	if v := os.Getenv(EnvVarUIURL); v != "" {
		set.UIURL = v
	}
	if v := os.Getenv(EnvVarAuthBase); v != "" {
		set.AuthBase = v
	}
	if v := os.Getenv(EnvVarClientID); v != "" {
		set.ClientID = v
	}
	if v := os.Getenv(EnvVarGraphQLURL); v != "" {
		set.GraphQLURL = v
	}
	if v := os.Getenv(EnvVarAudience); v != "" {
		set.Audience = v
	}
	if v := os.Getenv(EnvVarVaultGatewayURL); v != "" {
		set.GatewayURL = v
	}
	// TODO for each contract - check if it's really a contract, not an EOA
	if v := os.Getenv(EnvVarWorkflowRegistryAddress); v != "" {
		set.WorkflowRegistryAddress = v
	}
	if v := os.Getenv(EnvVarCapabilitiesRegistryAddress); v != "" {
		set.CapabilitiesRegistryAddress = v
	}

	if v := os.Getenv(EnvVarWorkflowRegistryChainSelector); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			set.WorkflowRegistryChainSelector = n
		}
	}
	if v := os.Getenv(EnvVarCapabilitiesRegistryChainSelector); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			set.CapabilitiesRegistryChainSelector = n
		}
	}

	return &set
}

func New() (*EnvironmentSet, error) {
	ff, err := loadEmbeddedEnvironmentFile()
	if err != nil {
		return nil, err
	}
	envName := os.Getenv(EnvVarEnv)
	if envName == "" {
		envName = DefaultEnv
	}
	return NewEnvironmentSet(ff, envName), nil
}

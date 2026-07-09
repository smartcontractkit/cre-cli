package environments

import (
	"embed"
	"fmt"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const (
	EnvVarEnv = "CRE_CLI_ENV"

	EnvVarAuthBase        = "CRE_CLI_AUTH_BASE"
	EnvVarClientID        = "CRE_CLI_CLIENT_ID"
	EnvVarGraphQLURL      = "CRE_CLI_GRAPHQL_URL"
	EnvVarAudience        = "CRE_CLI_AUDIENCE"
	EnvVarVaultGatewayURL = "CRE_VAULT_DON_GATEWAY_URL"

	EnvVarWorkflowRegistryAddress          = "CRE_CLI_WORKFLOW_REGISTRY_ADDRESS"
	EnvVarWorkflowRegistryChainName        = "CRE_CLI_WORKFLOW_REGISTRY_CHAIN_NAME"
	EnvVarWorkflowRegistryChainExplorerURL = "CRE_CLI_WORKFLOW_REGISTRY_CHAIN_EXPLORER_URL"
	EnvVarDonFamily                        = "CRE_CLI_DON_FAMILY"

	DefaultEnv     = "PRODUCTION"
	StagingEnv     = "STAGING"
	DevelopmentEnv = "DEVELOPMENT"
)

//go:embed environments.yaml
var envFileContent embed.FS

type EnvironmentSet struct {
	EnvName string `yaml:"-"`

	AuthBase   string `yaml:"CRE_CLI_AUTH_BASE"`
	ClientID   string `yaml:"CRE_CLI_CLIENT_ID"`
	GraphQLURL string `yaml:"CRE_CLI_GRAPHQL_URL"`
	Audience   string `yaml:"CRE_CLI_AUDIENCE"`
	GatewayURL string `yaml:"CRE_VAULT_DON_GATEWAY_URL"`

	WorkflowRegistryAddress          string `yaml:"CRE_CLI_WORKFLOW_REGISTRY_ADDRESS"`
	WorkflowRegistryChainName        string `yaml:"CRE_CLI_WORKFLOW_REGISTRY_CHAIN_NAME"`
	WorkflowRegistryChainExplorerURL string `yaml:"CRE_CLI_WORKFLOW_REGISTRY_CHAIN_EXPLORER_URL"`
	DonFamily                        string `yaml:"-"`
}

// RequiresVPN returns true if the GraphQL endpoint is on a private network
// (e.g. Tailscale) that requires VPN connectivity.
func (e *EnvironmentSet) RequiresVPN() bool {
	return strings.Contains(e.GraphQLURL, ".ts.net")
}

// EnvLabel returns the environment name for display purposes.
// Returns "" for the default (PRODUCTION) environment so callers can
// skip environment labeling when the user is in the standard context.
func (e *EnvironmentSet) EnvLabel() string {
	if e.EnvName == "" || e.EnvName == DefaultEnv {
		return ""
	}
	return e.EnvName
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

var newEnvironmentSetWarningsOnce sync.Once

func NewEnvironmentSet(ff *fileFormat, envName string) *EnvironmentSet {
	set, ok := ff.Envs[envName]
	if !ok {
		set = ff.Envs[DefaultEnv]
	}

	authBase := os.Getenv(EnvVarAuthBase)
	clientID := os.Getenv(EnvVarClientID)
	graphqlURL := os.Getenv(EnvVarGraphQLURL)
	audience := os.Getenv(EnvVarAudience)
	gatewayURL := os.Getenv(EnvVarVaultGatewayURL)
	wrChainExplorerURL := os.Getenv(EnvVarWorkflowRegistryChainExplorerURL)
	// TODO for workflow registry contract - check if it's really a contract, not an EOA
	wrAddress := os.Getenv(EnvVarWorkflowRegistryAddress)
	wrChainName := os.Getenv(EnvVarWorkflowRegistryChainName)
	donFamily := os.Getenv(EnvVarDonFamily)

	set.EnvName = envName
	if authBase != "" {
		set.AuthBase = authBase
	}
	if clientID != "" {
		set.ClientID = clientID
	}
	if graphqlURL != "" {
		set.GraphQLURL = graphqlURL
	}
	if audience != "" {
		set.Audience = audience
	}
	if gatewayURL != "" {
		set.GatewayURL = gatewayURL
	}
	if wrChainExplorerURL != "" {
		set.WorkflowRegistryChainExplorerURL = wrChainExplorerURL
	}
	if wrAddress != "" {
		set.WorkflowRegistryAddress = wrAddress
	}
	if wrChainName != "" {
		set.WorkflowRegistryChainName = wrChainName
	}
	if donFamily != "" {
		set.DonFamily = donFamily
	}

	newEnvironmentSetWarningsOnce.Do(func() {
		switch envName {
		case DefaultEnv:
		case DevelopmentEnv, StagingEnv:
			ui.Warning(fmt.Sprintf("%s set, using %s environment", EnvVarEnv, envName))
		default:
			ui.Warning(fmt.Sprintf("Environment %s not found, defaulting to %s", envName, DefaultEnv))
		}
		if authBase != "" {
			ui.Warning(fmt.Sprintf("%s set, using %s", EnvVarAuthBase, authBase))
		}
		if clientID != "" {
			ui.Warning(fmt.Sprintf("%s set, using %s", EnvVarClientID, clientID))
		}
		if graphqlURL != "" {
			ui.Warning(fmt.Sprintf("%s set, using %s", EnvVarGraphQLURL, graphqlURL))
		}
		if audience != "" {
			ui.Warning(fmt.Sprintf("%s set, using %s", EnvVarAudience, audience))
		}
		if gatewayURL != "" {
			ui.Warning(fmt.Sprintf("%s set, using %s", EnvVarVaultGatewayURL, gatewayURL))
		}
		if wrChainExplorerURL != "" {
			ui.Warning(fmt.Sprintf("%s set, using %s", EnvVarWorkflowRegistryChainExplorerURL, wrChainExplorerURL))
		}
		if wrAddress != "" {
			ui.Warning(fmt.Sprintf("%s set, using %s", EnvVarWorkflowRegistryAddress, wrAddress))
		}
		if wrChainName != "" {
			ui.Warning(fmt.Sprintf("%s set, using %s", EnvVarWorkflowRegistryChainName, wrChainName))
		}
		if donFamily != "" {
			ui.Warning(fmt.Sprintf("%s set, using %s", EnvVarDonFamily, donFamily))
		}
	})

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

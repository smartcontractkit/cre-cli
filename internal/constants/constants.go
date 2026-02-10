package constants

import (
	"time"

	chainselectors "github.com/smartcontractkit/chain-selectors"
)

const (
	WorkflowRegistryContractName      = "WorkflowRegistry"
	BalanceReaderContractName         = "BalanceReader"
	WERC20MockContractName            = "WERC20Mock"
	ReserveManagerContractName        = "ReserveManager"
	MockKeystoneForwarderContractName = "MockKeystoneForwarder"

	MaxSecretItemsPerPayload                    = 10
	MaxVaultAllowlistDuration     time.Duration = 7 * 24 * time.Hour
	DefaultVaultAllowlistDuration time.Duration = 2 * 24 * time.Hour // 2 days

	DefaultSethLogLevel = "error"

	// Default Values
	DefaultSethConfigPath = "\"\""
	DefaultTimelockDelay  = "0s"

	// Default settings
	DefaultProposalExpirationTime = 60 * 60 * 24 * 3 // 72 hours

	DefaultEthSepoliaRpcUrl = "https://ethereum-sepolia-rpc.publicnode.com" // ETH Sepolia
	DefaultEthMainnetRpcUrl = "<select your own rpc url>"                   // ETH Mainnet

	DefaultProjectName  = "my-project"
	DefaultWorkflowName = "my-workflow"

	DefaultProjectSettingsFileName  = "project.yaml"
	DefaultWorkflowSettingsFileName = "workflow.yaml"
	DefaultEnvFileName              = ".env"
	DefaultIsGoFileName             = "go.mod"

	AuthAuthorizePath = "/authorize"
	AuthTokenPath     = "/oauth/token"
	AuthRevokePath    = "/oauth/revoke"
	AuthBrowserLogout = "/v2/logout"

	AuthRedirectURI = "http://localhost:53682/callback"
	AuthListenAddr  = "localhost:53682"
	CreUiAuthPath   = "/auth/cli"

	WorkflowOwnerTypeEOA  = "EOA"
	WorkflowOwnerTypeMSIG = "MSIG"

	WorkflowRegistryV2TypeAndVersion = "WorkflowRegistry 2.0.0"

	WorkflowLanguageGolang     = "golang"
	WorkflowLanguageTypeScript = "typescript"

	// SDK dependency versions (used by generate-bindings and go module init)
	SdkVersion              = "v1.2.0"
	EVMCapabilitiesVersion  = "v1.0.0-beta.5"
	HTTPCapabilitiesVersion = "v1.0.0-beta.0"
	CronCapabilitiesVersion = "v1.0.0-beta.0"

	TestAddress      = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	TestAddress2     = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
	TestAddress3     = "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
	TestAddress4     = "0x90F79bf6EB2c4f870365E785982E1f101E93b906"
	TestPrivateKey   = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	TestPrivateKey2  = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
	TestPrivateKey3  = "5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a"
	TestPrivateKey4  = "7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6"
	TestAnvilChainID = 31337 // Anvil chain ID
)

var (
	DefaultEthMainnetChainName = chainselectors.ETHEREUM_MAINNET.Name
	DefaultEthSepoliaChainName = chainselectors.ETHEREUM_TESTNET_SEPOLIA.Name
)

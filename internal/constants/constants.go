package constants

import (
	"time"
)

const (
	// TODO cleanup this later, we will not need all of these env vars with Viper

	// Contract Names
	WorkflowRegistryContractName     = "WorkflowRegistry"
	CapabilitiesRegistryContractName = "CapabilitiesRegistry"

	// Limits
	MaxBinarySize                               = 20 * 1024 * 1024
	MaxConfigSize                               = 5 * 1024 * 1024
	MaxEncryptedSecretsSize                     = 5 * 1024 * 1024
	MaxURLLength                                = 200
	MaxPaginationLimit            uint32        = 100
	MaxVaultAllowlistDuration     time.Duration = 7 * 24 * time.Hour
	DefaultVaultAllowlistDuration time.Duration = 2 * 24 * time.Hour // 90 days

	// Logging Levels
	DefaultSethLogLevel = "error"

	// Default Values
	DefaultSethConfigPath = "\"\""
	DefaultTimelockDelay  = "0s"

	// Default settings
	DefaultProposalExpirationTime = 60 * 60 * 24 * 3 // 72 hours

	DefaultEthSepoliaChainSelector  = "16015286601757825753" // ETH Sepolia
	DefaultBaseSepoliaChainSelector = "10344971235874465080" // Base Sepolia
	DefaultEthMainnetChainSelector  = "5009297550715157269"  // Eth Mainnet

	DefaultEthSepoliaRpcUrl  = "https://sepolia.infura.io/v3/YOUR_API_KEY" // ETH Sepolia
	DefaultEthMainnetRpcUrl  = "<select your own rpc url>"                 // ETH Mainnet
	DefaultBaseSepoliaRpcUrl = "<select your own rpc url>"                 // Base Sepolia

	DefaultStagingDonFamily           = "zone-a" // Keystone team has to define this
	DefaultProductionTestnetDonFamily = "zone-a" // Keystone team has to define this
	DefaultProductionDonFamily        = "zone-a" // Keystone team has to define this

	DefaultProjectSettingsFileName  = "project.yaml"
	DefaultWorkflowSettingsFileName = "workflow.yaml"
	DefaultEnvFileName              = ".env"

	CognitoTokenExchangePath = "/oauth2/token"
	CognitoAuthRevokePath    = "/oauth2/revoke"
	CognitoAuthRedirectURI   = "http://localhost:53682/callback"
	CognitoAuthListenAddr    = "localhost:53682"
	CreUiAuthPath            = "/auth/cli"

	WorkflowOwnerTypeEOA  = "EOA"
	WorkflowOwnerTypeMSIG = "MSIG"

	WorkflowRegistryV2TypeAndVersion = "WorkflowRegistry 2.0.0"

	TestAddress      = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	TestAddress2     = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
	TestAddress3     = "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
	TestPrivateKey   = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	TestPrivateKey2  = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
	TestPrivateKey3  = "5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a"
	TestAnvilChainID = 31337 // Anvil chain ID
)

package deploy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/prompt"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

const (
	contractsFolder       = "contracts"
	contractsConfigFile   = "contracts.yaml"
	deployedContractsFile = "deployed_contracts.yaml"
)

type Inputs struct {
	ProjectRoot      string `validate:"required,dir"`
	ContractsPath    string `validate:"required,dir"`
	ConfigPath       string `validate:"required,file"`
	OutputPath       string `validate:"required"`
	ChainOverride    string `validate:"omitempty"`
	DryRun           bool
	SkipConfirmation bool
}

type handler struct {
	log            *zerolog.Logger
	v              *viper.Viper
	settings       *settings.Settings
	inputs         Inputs
	stdin          io.Reader
	runtimeContext *runtime.Context
	validated      bool
	config         *ContractsConfig
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var deployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Deploys smart contracts to the blockchain",
		Long: `Deploys smart contracts defined in contracts/contracts.yaml to the target blockchain.
The deployed contract addresses are stored in contracts/deployed_contracts.yaml
and can be referenced in workflow configurations using placeholders.`,
		Example: `  cre contract deploy
  cre contract deploy --dry-run
  cre contract deploy --chain ethereum-testnet-sepolia`,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := newHandler(runtimeContext, cmd.InOrStdin())

			inputs, err := h.ResolveInputs(runtimeContext.Viper)
			if err != nil {
				return err
			}
			h.inputs = inputs

			if err := h.ValidateInputs(); err != nil {
				return err
			}
			return h.Execute()
		},
	}

	deployCmd.Flags().Bool("dry-run", false, "Validate configuration without deploying contracts")
	deployCmd.Flags().String("chain", "", "Override the target chain from contracts.yaml")
	settings.AddSkipConfirmation(deployCmd)

	return deployCmd
}

func newHandler(ctx *runtime.Context, stdin io.Reader) *handler {
	return &handler{
		log:            ctx.Logger,
		v:              ctx.Viper,
		settings:       ctx.Settings,
		stdin:          stdin,
		runtimeContext: ctx,
		validated:      false,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	projectRoot := h.settings.ProjectRoot

	contractsPath := filepath.Join(projectRoot, contractsFolder)
	configPath := filepath.Join(contractsPath, contractsConfigFile)
	outputPath := filepath.Join(contractsPath, deployedContractsFile)

	return Inputs{
		ProjectRoot:      projectRoot,
		ContractsPath:    contractsPath,
		ConfigPath:       configPath,
		OutputPath:       outputPath,
		ChainOverride:    v.GetString("chain"),
		DryRun:           v.GetBool("dry-run"),
		SkipConfirmation: v.GetBool(settings.Flags.SkipConfirmation.Name),
	}, nil
}

func (h *handler) ValidateInputs() error {
	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	// Check contracts folder exists
	if _, err := os.Stat(h.inputs.ContractsPath); os.IsNotExist(err) {
		return fmt.Errorf("contracts folder not found at %s. Create a contracts/ folder in your project root", h.inputs.ContractsPath)
	}

	// Check contracts.yaml exists
	if _, err := os.Stat(h.inputs.ConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("contracts.yaml not found at %s. Create a contracts.yaml file in your contracts/ folder", h.inputs.ConfigPath)
	}

	if err := validate.Struct(h.inputs); err != nil {
		return validate.ParseValidationErrors(err)
	}

	// Parse and validate the contracts config
	config, err := ParseContractsConfig(h.inputs.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to parse contracts.yaml: %w", err)
	}

	// Apply chain override if specified
	if h.inputs.ChainOverride != "" {
		config.Chain = h.inputs.ChainOverride
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid contracts.yaml: %w", err)
	}

	h.config = config
	h.validated = true
	return nil
}

func (h *handler) Execute() error {
	if !h.validated {
		return fmt.Errorf("inputs not validated")
	}

	// Check if forge is installed (required for compiling contracts)
	forgePath, err := h.checkForgeInstalled()
	if err != nil {
		return h.promptForgeInstall()
	}
	h.log.Debug().Str("forge", forgePath).Msg("Found forge installation")

	// Compile contracts using forge
	if err := h.compileContracts(); err != nil {
		return fmt.Errorf("failed to compile contracts: %w", err)
	}

	h.displayDeploymentDetails()

	if h.inputs.DryRun {
		fmt.Println("\n[DRY RUN] Configuration validated successfully. No contracts were deployed.")
		return nil
	}

	// Ask for confirmation before deploying
	if !h.inputs.SkipConfirmation {
		contractsToDeploy := h.config.GetContractsToDeploy()
		if len(contractsToDeploy) == 0 {
			fmt.Println("\nNo contracts marked for deployment.")
			return nil
		}

		confirm, err := prompt.YesNoPrompt(os.Stdin, fmt.Sprintf("Deploy %d contract(s) to %s?", len(contractsToDeploy), h.config.Chain))
		if err != nil {
			return err
		}
		if !confirm {
			return fmt.Errorf("deployment cancelled by user")
		}
	}

	// Deploy contracts
	results, err := h.deployContracts()
	if err != nil {
		return fmt.Errorf("failed to deploy contracts: %w", err)
	}

	// Write deployed contracts file
	if err := WriteDeployedContracts(h.inputs.OutputPath, h.config.Chain, results); err != nil {
		return fmt.Errorf("failed to write deployed contracts file: %w", err)
	}

	fmt.Printf("\n[OK] Contracts deployed successfully\n")
	fmt.Printf("Deployed addresses saved to: %s\n", h.inputs.OutputPath)

	return nil
}

func (h *handler) displayDeploymentDetails() {
	fmt.Printf("\nContract Deployment\n")
	fmt.Printf("===================\n")
	fmt.Printf("Project Root:    %s\n", h.inputs.ProjectRoot)
	fmt.Printf("Target Chain:    %s\n", h.config.Chain)
	fmt.Printf("Config File:     %s\n", h.inputs.ConfigPath)
	fmt.Printf("\nContracts:\n")

	for _, contract := range h.config.Contracts {
		status := "skip"
		if contract.Deploy {
			status = "deploy"
		}
		fmt.Printf("  - %s (%s): %s\n", contract.Name, contract.Package, status)
	}
}

func (h *handler) deployContracts() ([]DeploymentResult, error) {
	contractsToDeploy := h.config.GetContractsToDeploy()
	if len(contractsToDeploy) == 0 {
		return nil, nil
	}

	// Get RPC URL for the target chain
	rpcURL, err := h.getRPCForChain(h.config.Chain)
	if err != nil {
		return nil, fmt.Errorf("failed to get RPC URL for chain %s: %w", h.config.Chain, err)
	}

	// Create eth client using the existing NewEthClientFromEnv function
	ethClient, err := client.NewEthClientFromEnv(h.v, h.log, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create eth client: %w", err)
	}

	// Load contract bindings into the client's ContractStore (from chainlink-evm)
	if err := client.LoadContracts(h.log, ethClient); err != nil {
		return nil, fmt.Errorf("failed to load contract bindings: %w", err)
	}

	// Load local contracts from project's ABI/BIN files
	abiDir := filepath.Join(h.inputs.ContractsPath, "evm", "src", "abi")
	if err := h.loadLocalContracts(ethClient, abiDir, contractsToDeploy); err != nil {
		h.log.Warn().Err(err).Msg("Failed to load some local contracts")
	}

	deployer := NewContractDeployer(h.log, ethClient, h.inputs.ContractsPath)

	var results []DeploymentResult
	for _, contract := range contractsToDeploy {
		fmt.Printf("\nDeploying %s...\n", contract.Name)

		result, err := deployer.Deploy(contract)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy %s: %w", contract.Name, err)
		}

		results = append(results, *result)
		fmt.Printf("  Address: %s\n", result.Address)
		fmt.Printf("  Tx Hash: %s\n", result.TxHash)
	}

	return results, nil
}

// loadLocalContracts loads contracts from local ABI and BIN files into the ContractStore
func (h *handler) loadLocalContracts(ethClient *seth.Client, abiDir string, contracts []ContractConfig) error {
	for _, contract := range contracts {
		// Skip if already loaded (from chainlink-evm)
		if _, ok := ethClient.ContractStore.GetABI(contract.Name); ok {
			h.log.Debug().Str("contract", contract.Name).Msg("Contract already loaded from chainlink-evm")
			continue
		}

		// Try to load from local ABI and BIN files
		abiPath := filepath.Join(abiDir, contract.Name+".abi")
		binPath := filepath.Join(abiDir, contract.Name+".bin")

		// Read ABI file
		abiData, err := os.ReadFile(abiPath)
		if err != nil {
			h.log.Debug().Str("contract", contract.Name).Err(err).Msg("No local ABI file found")
			continue
		}

		// Parse ABI
		contractABI, err := abi.JSON(strings.NewReader(string(abiData)))
		if err != nil {
			h.log.Warn().Str("contract", contract.Name).Err(err).Msg("Failed to parse ABI")
			continue
		}

		// Read BIN file (bytecode)
		binData, err := os.ReadFile(binPath)
		if err != nil {
			h.log.Warn().Str("contract", contract.Name).Err(err).Msg("No BIN file found - contract cannot be deployed. Compile your Solidity contracts to generate .bin files")
			continue
		}

		// Add to ContractStore
		ethClient.ContractStore.AddABI(contract.Name, contractABI)
		ethClient.ContractStore.AddBIN(contract.Name, common.FromHex(string(binData)))
		h.log.Debug().Str("contract", contract.Name).Msg("Loaded contract from local ABI/BIN files")
	}

	return nil
}

func (h *handler) getRPCForChain(chainName string) (string, error) {
	if h.settings == nil || len(h.settings.Workflow.RPCs) == 0 {
		return "", fmt.Errorf("no RPC endpoints configured. Add RPCs to your project.yaml")
	}

	for _, rpc := range h.settings.Workflow.RPCs {
		if rpc.ChainName == chainName {
			return rpc.Url, nil
		}
	}

	return "", fmt.Errorf("no RPC URL configured for chain %s", chainName)
}

// checkForgeInstalled checks if Foundry's forge is installed and available in PATH
func (h *handler) checkForgeInstalled() (string, error) {
	forgePath, err := exec.LookPath("forge")
	if err != nil {
		return "", fmt.Errorf("forge not found in PATH")
	}
	return forgePath, nil
}

// promptForgeInstall displays installation instructions for Foundry
func (h *handler) promptForgeInstall() error {
	fmt.Println("\n⚠️  Foundry (forge) is required to compile smart contracts")
	fmt.Println("\nFoundry is a blazing fast toolkit for Ethereum development.")
	fmt.Println("\nTo install Foundry, run:")
	fmt.Println("\n  curl -L https://foundry.paradigm.xyz | bash")
	fmt.Println("  foundryup")
	fmt.Println("\nFor more information, visit: https://book.getfoundry.sh/getting-started/installation")
	fmt.Println("\nAfter installation, run 'cre contract deploy' again.")
	return fmt.Errorf("forge is required but not installed")
}

// compileContracts runs forge build to compile Solidity contracts
func (h *handler) compileContracts() error {
	evmDir := filepath.Join(h.inputs.ContractsPath, "evm")

	// Check if evm directory exists (indicates Solidity contracts are present)
	if _, err := os.Stat(evmDir); os.IsNotExist(err) {
		h.log.Debug().Msg("No evm directory found, skipping compilation")
		return nil
	}

	// Check if there are any .sol files
	solFiles, err := filepath.Glob(filepath.Join(evmDir, "src", "*.sol"))
	if err != nil || len(solFiles) == 0 {
		h.log.Debug().Msg("No Solidity files found, skipping compilation")
		return nil
	}

	fmt.Println("\nCompiling contracts with Foundry...")

	// Run forge build
	cmd := exec.Command("forge", "build")
	cmd.Dir = evmDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("forge build failed: %w", err)
	}

	fmt.Println("Compilation successful!")

	// Extract bytecode from compiled artifacts
	if err := h.extractBytecode(evmDir); err != nil {
		return fmt.Errorf("failed to extract bytecode: %w", err)
	}

	return nil
}

// forgeArtifact represents the structure of a Forge compilation artifact
type forgeArtifact struct {
	Bytecode struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}

// extractBytecode extracts bytecode from Forge compilation output to .bin files
func (h *handler) extractBytecode(evmDir string) error {
	outDir := filepath.Join(evmDir, "out")
	abiDir := filepath.Join(evmDir, "src", "abi")

	// Check if out directory exists
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		return fmt.Errorf("forge output directory not found at %s", outDir)
	}

	// Ensure abi directory exists
	if err := os.MkdirAll(abiDir, 0750); err != nil {
		return fmt.Errorf("failed to create abi directory: %w", err)
	}

	// Get contracts to deploy
	contractsToDeploy := h.config.GetContractsToDeploy()

	for _, contract := range contractsToDeploy {
		// Look for the compiled artifact
		artifactPath := filepath.Join(outDir, contract.Name+".sol", contract.Name+".json")
		if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
			h.log.Debug().Str("contract", contract.Name).Msg("No compiled artifact found, skipping bytecode extraction")
			continue
		}

		// Read the artifact
		artifactData, err := os.ReadFile(artifactPath)
		if err != nil {
			h.log.Warn().Str("contract", contract.Name).Err(err).Msg("Failed to read artifact")
			continue
		}

		// Parse the artifact
		var artifact forgeArtifact
		if err := json.Unmarshal(artifactData, &artifact); err != nil {
			h.log.Warn().Str("contract", contract.Name).Err(err).Msg("Failed to parse artifact")
			continue
		}

		// Extract bytecode (remove trailing newline if present)
		bytecode := strings.TrimSpace(artifact.Bytecode.Object)
		if bytecode == "" || bytecode == "0x" {
			h.log.Debug().Str("contract", contract.Name).Msg("No bytecode in artifact (might be an interface)")
			continue
		}

		// Write bytecode to .bin file
		binPath := filepath.Join(abiDir, contract.Name+".bin")
		if err := os.WriteFile(binPath, []byte(bytecode), 0600); err != nil {
			return fmt.Errorf("failed to write bytecode for %s: %w", contract.Name, err)
		}

		h.log.Debug().Str("contract", contract.Name).Str("path", binPath).Msg("Extracted bytecode")
	}

	return nil
}

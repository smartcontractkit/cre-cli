package generatebindings

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/creinit"
	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/bindings"
	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana_bindings"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	ProjectRoot string `validate:"required,dir" cli:"--project-root"`
	ChainFamily string `validate:"required,oneof=evm" cli:"--chain-family"`
	Language    string `validate:"required,oneof=go" cli:"--language"`
	// just keeping it simple for now
	AbiPath string `validate:"required,path_read" cli:"--abi"`
	PkgName string `validate:"required" cli:"--pkg"`
	OutPath string `validate:"required" cli:"--out"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	generateBindingsCmd := &cobra.Command{
		Use:   "generate-bindings <chain-family>",
		Short: "Generate bindings from contract ABI",
		Long: `This command generates bindings from contract ABI files.
Supports EVM chain family and Go language.
Each contract gets its own package subdirectory to avoid naming conflicts.
For example, IERC20.abi generates bindings in generated/ierc20/ package.`,
		Example: "  cre generate-bindings evm",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext)

			inputs, err := handler.ResolveInputs(args, runtimeContext.Viper)
			if err != nil {
				return err
			}
			err = handler.ValidateInputs(inputs)
			if err != nil {
				return err
			}
			return handler.Execute(inputs)
		},
	}

	generateBindingsCmd.Flags().StringP("project-root", "p", "", "Path to project root directory (defaults to current directory)")
	generateBindingsCmd.Flags().StringP("language", "l", "go", "Target language (go)")
	generateBindingsCmd.Flags().StringP("abi", "a", "", "Path to ABI directory (defaults to contracts/{chain-family}/src/abi/)")
	generateBindingsCmd.Flags().StringP("pkg", "k", "bindings", "Base package name (each contract gets its own subdirectory)")

	return generateBindingsCmd
}

type handler struct {
	log       *zerolog.Logger
	validated bool
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:       ctx.Logger,
		validated: false,
	}
}

func (h *handler) ResolveInputs(args []string, v *viper.Viper) (Inputs, error) {
	// Get current working directory as default project root
	currentDir, err := os.Getwd()
	if err != nil {
		return Inputs{}, fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Resolve project root with fallback to current directory
	projectRoot := v.GetString("project-root")
	if projectRoot == "" {
		projectRoot = currentDir
	}

	contractsPath := filepath.Join(projectRoot, "contracts")
	if _, err := os.Stat(contractsPath); err != nil {
		return Inputs{}, fmt.Errorf("contracts folder not found in project root: %s", contractsPath)
	}

	// Chain family is now a positional argument
	chainFamily := args[0]

	// Language defaults are handled by StringP
	language := v.GetString("language")

	// Resolve ABI path with fallback to contracts/{chainFamily}/src/abi/
	abiPath := v.GetString("abi")
	if abiPath == "" {
		abiPath = filepath.Join(projectRoot, "contracts", chainFamily, "src", "abi")
	}

	// Package name defaults are handled by StringP
	pkgName := v.GetString("pkg")

	// Output path is contracts/{chainFamily}/src/generated/ under projectRoot
	outPath := filepath.Join(projectRoot, "contracts", chainFamily, "src", "generated")

	return Inputs{
		ProjectRoot: projectRoot,
		ChainFamily: chainFamily,
		Language:    language,
		AbiPath:     abiPath,
		PkgName:     pkgName,
		OutPath:     outPath,
	}, nil
}

func (h *handler) ValidateInputs(inputs Inputs) error {
	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	if err = validate.Struct(inputs); err != nil {
		return validate.ParseValidationErrors(err)
	}

	// Additional validation for ABI path
	if _, err := os.Stat(inputs.AbiPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("ABI path does not exist: %s", inputs.AbiPath)
		}
		return fmt.Errorf("failed to access ABI path: %w", err)
	}

	// Validate that if AbiPath is a directory, it contains .abi files
	if info, err := os.Stat(inputs.AbiPath); err == nil && info.IsDir() {
		files, err := filepath.Glob(filepath.Join(inputs.AbiPath, "*.abi"))
		if err != nil {
			return fmt.Errorf("failed to check for ABI files in directory: %w", err)
		}
		if len(files) == 0 {
			return fmt.Errorf("no .abi files found in directory: %s", inputs.AbiPath)
		}
	}

	h.validated = true
	return nil
}

// contractNameToPackage converts contract names to valid Go package names
// Examples: IERC20 -> ierc20, ReserveManager -> reserve_manager, IReserveManager -> ireserve_manager
func contractNameToPackage(contractName string) string {
	if contractName == "" {
		return ""
	}

	var result []rune
	runes := []rune(contractName)

	for i, r := range runes {
		// Convert to lowercase
		if r >= 'A' && r <= 'Z' {
			lower := r - 'A' + 'a'

			// Add underscore before uppercase letters, but not:
			// - At the beginning (i == 0)
			// - If the previous character was also uppercase and this is followed by lowercase (e.g., "ERC" in "ERC20")
			// - If this is part of a sequence of uppercase letters at the beginning (e.g., "IERC20" -> "ierc20")
			if i > 0 {
				prevIsUpper := runes[i-1] >= 'A' && runes[i-1] <= 'Z'
				nextIsLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'

				// Add underscore if:
				// - Previous char was lowercase (CamelCase boundary)
				// - Previous char was uppercase but this char is followed by lowercase (end of acronym)
				if !prevIsUpper || (prevIsUpper && nextIsLower && i > 1) {
					result = append(result, '_')
				}
			}

			result = append(result, lower)
		} else {
			result = append(result, r)
		}
	}

	return string(result)
}

func (h *handler) processAbiDirectory(inputs Inputs) error {
	// Read all .abi files in the directory
	files, err := filepath.Glob(filepath.Join(inputs.AbiPath, "*.abi"))
	if err != nil {
		return fmt.Errorf("failed to find ABI files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no .abi files found in directory: %s", inputs.AbiPath)
	}

	packageNames := make(map[string]bool)
	for _, abiFile := range files {
		contractName := filepath.Base(abiFile)
		contractName = contractName[:len(contractName)-4]
		packageName := contractNameToPackage(contractName)
		if _, exists := packageNames[packageName]; exists {
			return fmt.Errorf("package name collision: multiple contracts would generate the same package name '%s' (contracts are converted to snake_case for package names). Please rename one of your contract files to avoid this conflict", packageName)
		}
		packageNames[packageName] = true
	}

	// Process each ABI file
	for _, abiFile := range files {
		// Extract contract name from filename (remove .abi extension)
		contractName := filepath.Base(abiFile)
		contractName = contractName[:len(contractName)-4] // Remove .abi extension

		// Convert contract name to package name
		packageName := contractNameToPackage(contractName)

		// Create per-contract output directory
		contractOutDir := filepath.Join(inputs.OutPath, packageName)
		if err := os.MkdirAll(contractOutDir, 0o755); err != nil {
			return fmt.Errorf("failed to create contract output directory %s: %w", contractOutDir, err)
		}

		// Create output file path in contract-specific directory
		outputFile := filepath.Join(contractOutDir, contractName+".go")

		fmt.Printf("Processing ABI file: %s, contract: %s, package: %s, output: %s\n", abiFile, contractName, packageName, outputFile)

		err = bindings.GenerateBindings(
			"", // combinedJSONPath - empty for now
			abiFile,
			packageName,  // Use contract-specific package name
			contractName, // Use contract name as type name
			outputFile,
		)
		if err != nil {
			return fmt.Errorf("failed to generate bindings for %s: %w", contractName, err)
		}
	}

	return nil
}

func (h *handler) processSingleAbi(inputs Inputs) error {
	// Extract contract name from ABI file path
	contractName := filepath.Base(inputs.AbiPath)
	if filepath.Ext(contractName) == ".abi" {
		contractName = contractName[:len(contractName)-4] // Remove .abi extension
	}

	// Convert contract name to package name
	packageName := contractNameToPackage(contractName)

	// Create per-contract output directory
	contractOutDir := filepath.Join(inputs.OutPath, packageName)
	if err := os.MkdirAll(contractOutDir, 0o755); err != nil {
		return fmt.Errorf("failed to create contract output directory %s: %w", contractOutDir, err)
	}

	// Create output file path in contract-specific directory
	outputFile := filepath.Join(contractOutDir, contractName+".go")

	fmt.Printf("Processing single ABI file: %s, contract: %s, package: %s, output: %s\n", inputs.AbiPath, contractName, packageName, outputFile)

	return bindings.GenerateBindings(
		"", // combinedJSONPath - empty for now
		inputs.AbiPath,
		packageName,  // Use contract-specific package name
		contractName, // Use contract name as type name
		outputFile,
	)
}

func (h *handler) Execute(inputs Inputs) error {
	fmt.Printf("GenerateBindings would be called here: projectRoot=%s, chainFamily=%s, language=%s, abiPath=%s, pkgName=%s, outPath=%s\n", inputs.ProjectRoot, inputs.ChainFamily, inputs.Language, inputs.AbiPath, inputs.PkgName, inputs.OutPath)

	// Validate language
	switch inputs.Language {
	case "go":
		// Language supported, continue
	default:
		return fmt.Errorf("unsupported language: %s", inputs.Language)
	}

	// Validate chain family and handle accordingly
	switch inputs.ChainFamily {
	case "evm":
		// Create output directory if it doesn't exist
		if err := os.MkdirAll(inputs.OutPath, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		// Check if ABI path is a directory or file
		info, err := os.Stat(inputs.AbiPath)
		if err != nil {
			return fmt.Errorf("failed to access ABI path: %w", err)
		}

		if info.IsDir() {
			if err := h.processAbiDirectory(inputs); err != nil {
				return err
			}
		} else {
			if err := h.processSingleAbi(inputs); err != nil {
				return err
			}
		}

		err = runCommand(inputs.ProjectRoot, "go", "get", "github.com/smartcontractkit/cre-sdk-go@"+creinit.SdkVersion)
		if err != nil {
			return err
		}
		err = runCommand(inputs.ProjectRoot, "go", "get", "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm@"+creinit.EVMCapabilitiesVersion)
		if err != nil {
			return err
		}
		if err = runCommand(inputs.ProjectRoot, "go", "mod", "tidy"); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported chain family: %s", inputs.ChainFamily)
	}
}

func runCommand(dir string, command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %s: %w", command, err)
	}

	return nil
}

type SolanaInputs struct {
	ProjectRoot string `validate:"required,dir" cli:"--project-root"`
	Language    string `validate:"required,oneof=go" cli:"--language"`
	// just keeping it simple for now
	IdlPath string `validate:"required,path_read" cli:"--idl"`
	// PkgName string `validate:"required" cli:"--pkg"`
	OutPath string `validate:"required" cli:"--out"`
}

func NewSolanaBindings(runtimeContext *runtime.Context) *cobra.Command {
	var generateBindingsCmd = &cobra.Command{
		Use:   "generate-bindings-solana",
		Short: "Generate bindings from contract IDL",
		Long: `This command generates bindings from contract IDL files.
Supports Solana chain family and Go language.
Each contract gets its own package subdirectory to avoid naming conflicts.
For example, data_storage.json generates bindings in generated/data_storage/ package.`,
		Example: "  cre generate-bindings-solana",
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext)

			inputs, err := handler.ResolveInputsSolana(args, runtimeContext.Viper)
			if err != nil {
				return err
			}
			err = handler.ValidateInputsSolana(inputs)
			if err != nil {
				return err
			}
			return handler.ExecuteSolana(inputs)
		},
	}

	generateBindingsCmd.Flags().StringP("project-root", "p", "", "Path to project root directory (defaults to current directory)")
	generateBindingsCmd.Flags().StringP("language", "l", "go", "Target language (go)")
	generateBindingsCmd.Flags().StringP("abi", "a", "", "Path to ABI directory (defaults to contracts/{chain-family}/src/abi/)")
	generateBindingsCmd.Flags().StringP("pkg", "k", "bindings", "Base package name (each contract gets its own subdirectory)")

	return generateBindingsCmd
}

func (h *handler) ResolveInputsSolana(args []string, v *viper.Viper) (SolanaInputs, error) {
	// Get current working directory as default project root
	currentDir, err := os.Getwd()
	if err != nil {
		return SolanaInputs{}, fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Resolve project root with fallback to current directory
	projectRoot := v.GetString("project-root")
	if projectRoot == "" {
		projectRoot = currentDir
	}

	contractsPath := filepath.Join(projectRoot, "contracts")
	if _, err := os.Stat(contractsPath); err != nil {
		return SolanaInputs{}, fmt.Errorf("contracts folder not found in project root: %s", contractsPath)
	}

	// Language defaults are handled by StringP
	language := v.GetString("language")

	// Resolve ABI path with fallback to contracts/{chainFamily}/src/abi/
	idlPath := v.GetString("idl")
	if idlPath == "" {
		idlPath = filepath.Join(projectRoot, "contracts", "solana", "src", "idl")
	}

	// Output path is contracts/{chainFamily}/src/generated/ under projectRoot
	outPath := filepath.Join(projectRoot, "contracts", "solana", "src", "generated")

	return SolanaInputs{
		ProjectRoot: projectRoot,
		Language:    language,
		IdlPath:     idlPath,
		OutPath:     outPath,
	}, nil
}

func (h *handler) ValidateInputsSolana(inputs SolanaInputs) error {
	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	if err = validate.Struct(inputs); err != nil {
		return validate.ParseValidationErrors(err)
	}

	// Additional validation for ABI path
	if _, err := os.Stat(inputs.IdlPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("IDL path does not exist: %s", inputs.IdlPath)
		}
		return fmt.Errorf("failed to access IDL path: %w", err)
	}

	// Validate that if AbiPath is a directory, it contains .abi files
	if info, err := os.Stat(inputs.IdlPath); err == nil && info.IsDir() {
		files, err := filepath.Glob(filepath.Join(inputs.IdlPath, "*.json"))
		if err != nil {
			return fmt.Errorf("failed to check for ABI files in directory: %w", err)
		}
		if len(files) == 0 {
			return fmt.Errorf("no .json files found in directory: %s", inputs.IdlPath)
		}
	}

	h.validated = true
	return nil
}

func (h *handler) processIdlDirectory(inputs SolanaInputs) error {
	// Read all .abi files in the directory
	files, err := filepath.Glob(filepath.Join(inputs.IdlPath, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to find ABI files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no .json files found in directory: %s", inputs.IdlPath)
	}

	// Process each ABI file
	for _, idlFile := range files {
		// Extract contract name from filename (remove .abi extension)
		contractName := filepath.Base(idlFile)
		contractName = contractName[:len(contractName)-5] // Remove .json extension

		// Create per-contract output directory
		contractOutDir := filepath.Join(inputs.OutPath, contractName)
		if err := os.MkdirAll(contractOutDir, 0755); err != nil {
			return fmt.Errorf("failed to create contract output directory %s: %w", contractOutDir, err)
		}

		// Create output file path in contract-specific directory
		outputFile := filepath.Join(contractOutDir, contractName+".go")

		fmt.Printf("Processing IDL file: %s, contract: %s, output: %s\n", idlFile, contractName, outputFile)

		err = solana_bindings.GenerateBindings(
			idlFile,
			contractName,
			contractOutDir,
		)
		if err != nil {
			return fmt.Errorf("failed to generate bindings for %s: %w", idlFile, err)
		}
	}

	return nil
}

func (h *handler) processSingleIdl(inputs SolanaInputs) error {
	// Extract contract name from ABI file path
	contractName := filepath.Base(inputs.IdlPath)
	if filepath.Ext(contractName) == ".json" {
		contractName = contractName[:len(contractName)-4] // Remove .json extension
	}

	// Create per-contract output directory
	contractOutDir := filepath.Join(inputs.OutPath, contractName)
	if err := os.MkdirAll(contractOutDir, 0755); err != nil {
		return fmt.Errorf("failed to create contract output directory %s: %w", contractOutDir, err)
	}

	fmt.Printf("Processing single IDL file: %s, contract: %s, output: %s\n", inputs.IdlPath, contractName, contractOutDir)

	return solana_bindings.GenerateBindings(
		inputs.IdlPath,
		contractName,
		contractOutDir,
	)
}

func (h *handler) ExecuteSolana(inputs SolanaInputs) error {
	fmt.Printf("GenerateBindings would be called here: projectRoot=%s, chainFamily=solana, language=%s, abiPath=%s, pkgName=%s, outPath=%s\n",
		inputs.ProjectRoot, inputs.Language, inputs.IdlPath, inputs.OutPath)

	// Validate language
	switch inputs.Language {
	case "go":
		// Language supported, continue
	default:
		return fmt.Errorf("unsupported language: %s", inputs.Language)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(inputs.OutPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if IDL path is a directory or file
	info, err := os.Stat(inputs.IdlPath)
	if err != nil {
		return fmt.Errorf("failed to access IDL path: %w", err)
	}

	if info.IsDir() {
		if err := h.processIdlDirectory(inputs); err != nil {
			return err
		}
	} else {
		if err := h.processSingleIdl(inputs); err != nil {
			return err
		}
	}

	// err = runCommand(inputs.ProjectRoot, "go", "get", "github.com/smartcontractkit/cre-sdk-go@"+creinit.SdkVersion)
	// if err != nil {
	// 	return err
	// }
	// err = runCommand(inputs.ProjectRoot, "go", "get", "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm@"+creinit.SdkVersion)
	// if err != nil {
	// 	return err
	// }
	// if err = runCommand(inputs.ProjectRoot, "go", "mod", "tidy"); err != nil {
	// 	return err
	// }
	return nil
}

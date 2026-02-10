package generatebindings

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/creinit"
	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/evm"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

func resolveEvmInputs(v *viper.Viper) (EvmInputs, error) {
	// Get current working directory as default project root
	currentDir, err := os.Getwd()
	if err != nil {
		return EvmInputs{}, fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Resolve project root with fallback to current directory
	projectRoot := v.GetString("project-root")
	if projectRoot == "" {
		projectRoot = currentDir
	}

	contractsPath := filepath.Join(projectRoot, "contracts")
	if _, err := os.Stat(contractsPath); err != nil {
		return EvmInputs{}, fmt.Errorf("contracts folder not found in project root: %s", contractsPath)
	}

	// Language defaults are handled by StringP
	language := v.GetString("language")

	// Resolve ABI path with fallback to contracts/{chainFamily}/src/abi/
	abiPath := v.GetString("abi")
	if abiPath == "" {
		abiPath = filepath.Join(projectRoot, "contracts", "evm", "src", "abi")
	}

	// Package name defaults are handled by StringP
	pkgName := v.GetString("pkg")

	// Output path is contracts/{chainFamily}/src/generated/ under projectRoot
	outPath := filepath.Join(projectRoot, "contracts", "evm", "src", "generated")

	return EvmInputs{
		ProjectRoot: projectRoot,
		Language:    language,
		AbiPath:     abiPath,
		PkgName:     pkgName,
		OutPath:     outPath,
	}, nil
}

func validateEvmInputs(inputs EvmInputs) error {
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

func processEvmAbiDirectory(inputs EvmInputs) error {
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

		err = evm.GenerateBindings(
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

func processEvmSingleAbi(inputs EvmInputs) error {
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

	return evm.GenerateBindings(
		"", // combinedJSONPath - empty for now
		inputs.AbiPath,
		packageName,  // Use contract-specific package name
		contractName, // Use contract name as type name
		outputFile,
	)
}

func executeEvm(inputs EvmInputs) error {
	// Validate language
	switch inputs.Language {
	case "go":
		// Language supported, continue
	default:
		return fmt.Errorf("unsupported language: %s", inputs.Language)
	}

	// Validate chain family and handle accordingly
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
		if err := processEvmAbiDirectory(inputs); err != nil {
			return err
		}
	} else {
		if err := processEvmSingleAbi(inputs); err != nil {
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
	err = runCommand(inputs.ProjectRoot, "go", "mod", "tidy")
	if err != nil {
		return err
	}
	return nil
}

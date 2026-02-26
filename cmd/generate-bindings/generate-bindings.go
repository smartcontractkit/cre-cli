package generatebindings

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/creinit"
	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/bindings"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	ProjectRoot string `validate:"required,dir" cli:"--project-root"`
	ChainFamily string `validate:"required,oneof=evm" cli:"--chain-family"`
	GoLang      bool   `cli:"--go"`
	TypeScript  bool   `cli:"--typescript"`
	AbiPath     string `validate:"required,path_read" cli:"--abi"`
	PkgName     string `validate:"required" cli:"--pkg"`
	GoOutPath   string // contracts/{chain}/src/generated — set when GoLang is true
	TSOutPath   string // contracts/{chain}/ts/generated — set when TypeScript is true
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
	generateBindingsCmd.Flags().Bool("go", false, "Generate Go bindings")
	generateBindingsCmd.Flags().Bool("typescript", false, "Generate TypeScript bindings")
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

func detectLanguages(projectRoot string) (goLang, typescript bool) {
	_ = filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Skip node_modules and other dependency directories
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		base := filepath.Base(path)
		if strings.HasSuffix(base, ".go") {
			goLang = true
		}
		if strings.HasSuffix(base, ".ts") && !strings.HasSuffix(base, ".d.ts") {
			typescript = true
		}
		return nil
	})
	return goLang, typescript
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

	// Resolve languages: flags take precedence, else auto-detect
	goLang := v.GetBool("go")
	typescript := v.GetBool("typescript")
	if !goLang && !typescript {
		goLang, typescript = detectLanguages(projectRoot)
	}
	if !goLang && !typescript {
		return Inputs{}, fmt.Errorf("no target language specified and none detected (use --go and/or --typescript, or ensure project contains .go or .ts files)")
	}

	// Unified ABI path for both languages: contracts/{chain}/src/abi
	abiPath := v.GetString("abi")
	if abiPath == "" {
		abiPath = filepath.Join(projectRoot, "contracts", chainFamily, "src", "abi")
	}

	// Package name defaults are handled by StringP
	pkgName := v.GetString("pkg")

	// Separate output paths: Go uses src/, TS uses ts/ (typescript convention)
	var goOutPath, tsOutPath string
	if goLang {
		goOutPath = filepath.Join(projectRoot, "contracts", chainFamily, "src", "generated")
	}
	if typescript {
		tsOutPath = filepath.Join(projectRoot, "contracts", chainFamily, "ts", "generated")
	}

	return Inputs{
		ProjectRoot: projectRoot,
		ChainFamily: chainFamily,
		GoLang:      goLang,
		TypeScript:  typescript,
		AbiPath:     abiPath,
		PkgName:     pkgName,
		GoOutPath:   goOutPath,
		TSOutPath:   tsOutPath,
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

	// Validate that if AbiPath is a directory, it contains ABI files (*.abi for both languages)
	if info, err := os.Stat(inputs.AbiPath); err == nil && info.IsDir() {
		abiExt := "*.abi"
		files, err := filepath.Glob(filepath.Join(inputs.AbiPath, abiExt))
		if err != nil {
			return fmt.Errorf("failed to check for ABI files in directory: %w", err)
		}
		if len(files) == 0 {
			return fmt.Errorf("no %s files found in directory: %s", abiExt, inputs.AbiPath)
		}
	}

	// Ensure at least one output path is set for the active language(s)
	if inputs.GoLang && inputs.GoOutPath == "" {
		return fmt.Errorf("Go output path is required when --go is set")
	}
	if inputs.TypeScript && inputs.TSOutPath == "" {
		return fmt.Errorf("TypeScript output path is required when --typescript is set")
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
	abiExt := "*.abi"
	files, err := filepath.Glob(filepath.Join(inputs.AbiPath, abiExt))
	if err != nil {
		return fmt.Errorf("failed to find ABI files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no %s files found in directory: %s", abiExt, inputs.AbiPath)
	}

	if inputs.GoLang {
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
	}

	// Track generated files for TypeScript barrel export
	var generatedContracts []string

	// Process each ABI file
	for _, abiFile := range files {
		contractName := filepath.Base(abiFile)
		ext := filepath.Ext(contractName)
		contractName = contractName[:len(contractName)-len(ext)]

		if inputs.TypeScript {
			outputFile := filepath.Join(inputs.TSOutPath, contractName+".ts")
			ui.Dim(fmt.Sprintf("Processing: %s -> %s", contractName, outputFile))

			err = bindings.GenerateBindingsTS(
				abiFile,
				contractName,
				outputFile,
			)
			if err != nil {
				return fmt.Errorf("failed to generate TypeScript bindings for %s: %w", contractName, err)
			}
			generatedContracts = append(generatedContracts, contractName)
		}

		if inputs.GoLang {
			packageName := contractNameToPackage(contractName)

			contractOutDir := filepath.Join(inputs.GoOutPath, packageName)
			if err := os.MkdirAll(contractOutDir, 0o755); err != nil {
				return fmt.Errorf("failed to create contract output directory %s: %w", contractOutDir, err)
			}

			outputFile := filepath.Join(contractOutDir, contractName+".go")
			ui.Dim(fmt.Sprintf("Processing: %s -> %s", contractName, outputFile))

			err = bindings.GenerateBindings(
				"",
				abiFile,
				packageName,
				contractName,
				outputFile,
			)
			if err != nil {
				return fmt.Errorf("failed to generate bindings for %s: %w", contractName, err)
			}
		}
	}

	// Generate barrel index.ts for TypeScript
	if inputs.TypeScript && len(generatedContracts) > 0 {
		indexPath := filepath.Join(inputs.TSOutPath, "index.ts")
		var indexContent string
		indexContent += "// Code generated — DO NOT EDIT.\n"
		for _, name := range generatedContracts {
			indexContent += fmt.Sprintf("export * from './%s'\n", name)
			indexContent += fmt.Sprintf("export * from './%s_mock'\n", name)
		}
		if err := os.WriteFile(indexPath, []byte(indexContent), 0o600); err != nil {
			return fmt.Errorf("failed to write index.ts: %w", err)
		}
	}

	return nil
}

func (h *handler) processSingleAbi(inputs Inputs) error {
	contractName := filepath.Base(inputs.AbiPath)
	ext := filepath.Ext(contractName)
	if ext != "" {
		contractName = contractName[:len(contractName)-len(ext)]
	}

	if inputs.TypeScript {
		outputFile := filepath.Join(inputs.TSOutPath, contractName+".ts")
		ui.Dim(fmt.Sprintf("Processing: %s -> %s", contractName, outputFile))

		if err := bindings.GenerateBindingsTS(
			inputs.AbiPath,
			contractName,
			outputFile,
		); err != nil {
			return err
		}
	}

	if inputs.GoLang {
		packageName := contractNameToPackage(contractName)

		contractOutDir := filepath.Join(inputs.GoOutPath, packageName)
		if err := os.MkdirAll(contractOutDir, 0o755); err != nil {
			return fmt.Errorf("failed to create contract output directory %s: %w", contractOutDir, err)
		}

		outputFile := filepath.Join(contractOutDir, contractName+".go")
		ui.Dim(fmt.Sprintf("Processing: %s -> %s", contractName, outputFile))

		if err := bindings.GenerateBindings(
			"",
			inputs.AbiPath,
			packageName,
			contractName,
			outputFile,
		); err != nil {
			return err
		}
	}

	return nil
}

func (h *handler) Execute(inputs Inputs) error {
	langs := []string{}
	if inputs.GoLang {
		langs = append(langs, "go")
	}
	if inputs.TypeScript {
		langs = append(langs, "typescript")
	}
	ui.Dim(fmt.Sprintf("Project: %s, Chain: %s, Languages: %v", inputs.ProjectRoot, inputs.ChainFamily, langs))

	// Validate chain family and handle accordingly
	switch inputs.ChainFamily {
	case "evm":
		// Create output directories for active language(s)
		if inputs.GoLang {
			if err := os.MkdirAll(inputs.GoOutPath, 0o755); err != nil {
				return fmt.Errorf("failed to create Go output directory: %w", err)
			}
		}
		if inputs.TypeScript {
			if err := os.MkdirAll(inputs.TSOutPath, 0o755); err != nil {
				return fmt.Errorf("failed to create TypeScript output directory: %w", err)
			}
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

		if inputs.GoLang {
			spinner := ui.NewSpinner()
			spinner.Start("Installing dependencies...")

			err = runCommand(inputs.ProjectRoot, "go", "get", "github.com/smartcontractkit/cre-sdk-go@"+creinit.SdkVersion)
			if err != nil {
				spinner.Stop()
				return err
			}
			err = runCommand(inputs.ProjectRoot, "go", "get", "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/evm@"+creinit.EVMCapabilitiesVersion)
			if err != nil {
				spinner.Stop()
				return err
			}
			if err = runCommand(inputs.ProjectRoot, "go", "mod", "tidy"); err != nil {
				spinner.Stop()
				return err
			}

			spinner.Stop()
		}

		ui.Success("Bindings generated successfully")
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

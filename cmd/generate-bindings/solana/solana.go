package solana

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/creinit"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	ProjectRoot string `validate:"required,dir" cli:"--project-root"`
	Language    string `validate:"required,oneof=go" cli:"--language"`
	IdlPath     string `validate:"required,path_read" cli:"--idl"`
	OutPath     string `validate:"required" cli:"--out"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var generateBindingsCmd = &cobra.Command{
		Use:   "solana",
		Short: "Generate bindings from contract IDL",
		Long: `This command generates bindings from contract IDL files.
Supports Solana chain family and Go language.
Each contract gets its own package subdirectory to avoid naming conflicts.
For example, data_storage.json generates bindings in generated/data_storage/ package.`,
		Example: "  cre generate-bindings-solana",
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext)
			inputs, err := handler.ResolveInputs(runtimeContext.Viper)
			if err != nil {
				return err
			}
			if err := handler.ValidateInputs(inputs); err != nil {
				return err
			}
			return handler.Execute(inputs)
		},
	}

	generateBindingsCmd.Flags().StringP("project-root", "p", "", "Path to project root directory (defaults to current directory)")
	generateBindingsCmd.Flags().StringP("language", "l", "go", "Target language (go)")
	generateBindingsCmd.Flags().StringP("idl", "i", "", "Path to IDL directory (defaults to contracts/solana/src/idl/)")
	generateBindingsCmd.Flags().StringP("out", "o", "", "Path to output directory (defaults to contracts/solana/src/generated/)")

	return generateBindingsCmd
}

type handler struct {
	log *zerolog.Logger
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log: ctx.Logger,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
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

	// Language defaults are handled by StringP
	language := v.GetString("language")

	// Resolve IDL path with fallback to contracts/solana/src/idl/
	idlPath := v.GetString("idl")
	if idlPath == "" {
		idlPath = filepath.Join(projectRoot, "contracts", "solana", "src", "idl")
	}

	// Output path is contracts/{chainFamily}/src/generated/ under projectRoot
	outPath := filepath.Join(projectRoot, "contracts", "solana", "src", "generated")

	return Inputs{
		ProjectRoot: projectRoot,
		Language:    language,
		IdlPath:     idlPath,
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

	// Additional validation for Idl path
	if _, err := os.Stat(inputs.IdlPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("IDL path does not exist: %s", inputs.IdlPath)
		}
		return fmt.Errorf("failed to access IDL path: %w", err)
	}

	// Validate that if IdlPath is a directory, it contains .json files
	if info, err := os.Stat(inputs.IdlPath); err == nil && info.IsDir() {
		files, err := filepath.Glob(filepath.Join(inputs.IdlPath, "*.json"))
		if err != nil {
			return fmt.Errorf("failed to check for IDL files in directory: %w", err)
		}
		if len(files) == 0 {
			return fmt.Errorf("no .json files found in directory: %s", inputs.IdlPath)
		}
	}

	return nil
}

func (h *handler) processIdlDirectory(inputs Inputs) error {
	// Read all .json files in the directory
	files, err := filepath.Glob(filepath.Join(inputs.IdlPath, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to find IDL files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no .json files found in directory: %s", inputs.IdlPath)
	}

	// Process each IDL file
	for _, idlFile := range files {
		// Extract contract name from filename (remove .json extension)
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

		err = GenerateBindings(
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

func (h *handler) processSingleIdl(inputs Inputs) error {
	// Extract contract name from IDL file path
	contractName := filepath.Base(inputs.IdlPath)
	if filepath.Ext(contractName) == ".json" {
		contractName = contractName[:len(contractName)-5] // Remove .json extension
	}

	// Create per-contract output directory
	contractOutDir := filepath.Join(inputs.OutPath, contractName)
	if err := os.MkdirAll(contractOutDir, 0755); err != nil {
		return fmt.Errorf("failed to create contract output directory %s: %w", contractOutDir, err)
	}

	fmt.Printf("Processing single IDL file: %s, contract: %s, output: %s\n", inputs.IdlPath, contractName, contractOutDir)

	return GenerateBindings(
		inputs.IdlPath,
		contractName,
		contractOutDir,
	)
}

func (h *handler) Execute(inputs Inputs) error {
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

	err = runCommand(inputs.ProjectRoot, "go", "get", "github.com/smartcontractkit/cre-sdk-go@"+creinit.SdkVersion)
	if err != nil {
		return err
	}
	err = runCommand(inputs.ProjectRoot, "go", "get", "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana@"+creinit.SolanaCapabilitiesVersion)
	if err != nil {
		return err
	}
	err = runCommand(inputs.ProjectRoot, "go", "mod", "tidy")
	if err != nil {
		return err
	}

	return nil
}

// runCommand executes a command in a specified directory
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

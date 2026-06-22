package solana

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	ProjectRoot string `validate:"required,dir" cli:"--project-root"`
	GoLang      bool
	TypeScript  bool
	IdlPath     string `validate:"required,path_read" cli:"--idl"`
	GoOutPath   string // contracts/solana/src/generated — set when GoLang is true
	TSOutPath   string // contracts/solana/ts/generated — set when TypeScript is true
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var generateBindingsCmd = &cobra.Command{
		Use:   "solana",
		Short: "Generate bindings from contract IDL",
		Long: `This command generates bindings from contract IDL files.
Supports Solana chain family with Go and TypeScript languages.
The target language is auto-detected from project files, or can be
specified explicitly with --language.
For Go, each contract gets its own package subdirectory to avoid naming
conflicts: data_storage.json generates bindings in generated/data_storage/.
For TypeScript, each contract generates a flat <Program>.ts + <Program>_mock.ts
pair plus an index.ts barrel.`,
		Example: "  cre generate-bindings solana",
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
	generateBindingsCmd.Flags().StringP("language", "l", "", "Target language: go, typescript (auto-detected from project files when omitted)")
	generateBindingsCmd.Flags().StringP("idl", "i", "", "Path to IDL directory (defaults to contracts/solana/src/idl/)")
	generateBindingsCmd.Flags().StringP("out", "o", "", "Path to output directory (defaults to contracts/solana/src/generated/ for Go, contracts/solana/ts/generated/ for TypeScript)")

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

// detectLanguages mirrors the EVM generate-bindings auto-detection: a project
// containing .go files targets Go, one containing .ts files targets TypeScript.
func detectLanguages(projectRoot string) (goLang, typescript bool) {
	_ = filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
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
		if goLang && typescript {
			return filepath.SkipAll
		}
		return nil
	})
	return goLang, typescript
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

	// Resolve languages: --language flag takes precedence, else auto-detect
	var goLang, typescript bool
	langFlag := strings.ToLower(strings.TrimSpace(v.GetString("language")))
	switch langFlag {
	case "":
		goLang, typescript = detectLanguages(projectRoot)
		if !goLang && !typescript {
			return Inputs{}, fmt.Errorf("no target language detected (use --language go or --language typescript, or ensure project contains .go or .ts files)")
		}
	case constants.WorkflowLanguageGolang:
		goLang = true
	case constants.WorkflowLanguageTypeScript:
		typescript = true
	default:
		return Inputs{}, fmt.Errorf("unsupported language %q (supported: go, typescript)", langFlag)
	}

	// Resolve IDL path with fallback to contracts/solana/src/idl/
	idlPath := v.GetString("idl")
	if idlPath == "" {
		idlPath = filepath.Join(projectRoot, "contracts", "solana", "src", "idl")
	}

	// Separate output paths: Go uses src/, TS uses ts/ (typescript convention).
	// --out overrides the default for the selected language; with both
	// languages selected it would be ambiguous, so it is rejected.
	outFlag := v.GetString("out")
	if outFlag != "" && goLang && typescript {
		return Inputs{}, fmt.Errorf("--out is ambiguous when generating for both go and typescript; use --language to select one")
	}

	var goOutPath, tsOutPath string
	if goLang {
		goOutPath = outFlag
		if goOutPath == "" {
			goOutPath = filepath.Join(projectRoot, "contracts", "solana", "src", "generated")
		}
	}
	if typescript {
		tsOutPath = outFlag
		if tsOutPath == "" {
			tsOutPath = filepath.Join(projectRoot, "contracts", "solana", "ts", "generated")
		}
	}

	return Inputs{
		ProjectRoot: projectRoot,
		GoLang:      goLang,
		TypeScript:  typescript,
		IdlPath:     idlPath,
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

// contractNameFromIdlFile returns the contract name by stripping the .json
// extension from the base filename.
func contractNameFromIdlFile(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".json")
}

func (h *handler) generateForIdl(inputs Inputs, idlFile string) (className string, err error) {
	contractName := contractNameFromIdlFile(idlFile)

	if inputs.TypeScript {
		ui.Dim(fmt.Sprintf("Processing IDL file: %s, contract: %s, output: %s\n", idlFile, contractName, inputs.TSOutPath))
		className, err = GenerateBindingsTS(
			idlFile,
			contractName,
			inputs.TSOutPath,
		)
		if err != nil {
			return "", fmt.Errorf("failed to generate TypeScript bindings for %s: %w", idlFile, err)
		}
	}

	if inputs.GoLang {
		// Create per-contract output directory
		contractOutDir := filepath.Join(inputs.GoOutPath, contractName)
		if err := os.MkdirAll(contractOutDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create contract output directory %s: %w", contractOutDir, err)
		}

		ui.Dim(fmt.Sprintf("Processing IDL file: %s, contract: %s, output: %s\n", idlFile, contractName, contractOutDir))

		if err := GenerateBindings(
			idlFile,
			contractName,
			contractOutDir,
		); err != nil {
			return "", fmt.Errorf("failed to generate bindings for %s: %w", idlFile, err)
		}
	}

	return className, nil
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

	var generatedTSClasses []string
	for _, idlFile := range files {
		className, err := h.generateForIdl(inputs, idlFile)
		if err != nil {
			return err
		}
		if className != "" {
			generatedTSClasses = append(generatedTSClasses, className)
		}
	}

	return h.writeTSBarrel(inputs, generatedTSClasses)
}

func (h *handler) processSingleIdl(inputs Inputs) error {
	className, err := h.generateForIdl(inputs, inputs.IdlPath)
	if err != nil {
		return err
	}
	var classNames []string
	if className != "" {
		classNames = []string{className}
	}
	return h.writeTSBarrel(inputs, classNames)
}

// writeTSBarrel generates the index.ts barrel re-exporting every generated
// binding and mock, matching the EVM TypeScript output convention.
func (h *handler) writeTSBarrel(inputs Inputs, classNames []string) error {
	if !inputs.TypeScript || len(classNames) == 0 {
		return nil
	}
	indexPath := filepath.Join(inputs.TSOutPath, "index.ts")
	var sb strings.Builder
	sb.WriteString("// Code generated — DO NOT EDIT.\n")
	for _, name := range classNames {
		fmt.Fprintf(&sb, "export * from './%s'\nexport * from './%s_mock'\n", name, name)
	}
	if err := os.WriteFile(indexPath, []byte(sb.String()), 0o600); err != nil {
		return fmt.Errorf("failed to write index.ts: %w", err)
	}
	return nil
}

func (h *handler) Execute(inputs Inputs) error {
	var langs []string
	if inputs.GoLang {
		langs = append(langs, "go")
	}
	if inputs.TypeScript {
		langs = append(langs, "typescript")
	}
	ui.Dim(fmt.Sprintf("Project: %s, Chain: solana, Languages: %v", inputs.ProjectRoot, langs))

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

	if inputs.GoLang {
		err = runCommand(inputs.ProjectRoot, "go", "get", "github.com/smartcontractkit/cre-sdk-go@"+constants.SdkVersion)
		if err != nil {
			return err
		}
		err = runCommand(inputs.ProjectRoot, "go", "get", "github.com/smartcontractkit/cre-sdk-go/capabilities/blockchain/solana@"+constants.SolanaCapabilitiesVersion)
		if err != nil {
			return err
		}
		err = runCommand(inputs.ProjectRoot, "go", "mod", "tidy")
		if err != nil {
			return err
		}
	}

	ui.Success("Bindings generated successfully")
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

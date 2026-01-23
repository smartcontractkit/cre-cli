package convert

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/prompt"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

const (
	wasmWorkflowPath = "./wasm/workflow.wasm"
	convertWarning   = "This will convert your workflow to a custom (self-compiled) build. This cannot be undone by the CLI. Continue?"
)

type Inputs struct {
	WorkflowFolder string
	Force         bool
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var force bool
	convertCmd := &cobra.Command{
		Use:     "convert-to-custom-build <workflow-folder-path>",
		Short:   "Converts an existing workflow to a custom (self-compiled) build",
		Long:    `Converts a Go or TypeScript workflow to use a custom build via Makefile, producing wasm/workflow.wasm. The workflow-path in workflow.yaml is updated to ./wasm/workflow.wasm. This cannot be undone.`,
		Args:    cobra.ExactArgs(1),
		Example: `cre workflow convert-to-custom-build ./my-workflow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext, cmd.InOrStdin())
			inputs := Inputs{
				WorkflowFolder: args[0],
				Force:         force,
			}
			return handler.Execute(inputs)
		},
	}
	convertCmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt and convert immediately")
	return convertCmd
}

type handler struct {
	log            *zerolog.Logger
	stdin          io.Reader
	runtimeContext *runtime.Context
}

func newHandler(runtimeContext *runtime.Context, stdin io.Reader) *handler {
	h := &handler{stdin: stdin, runtimeContext: runtimeContext}
	if runtimeContext != nil {
		h.log = runtimeContext.Logger
	}
	return h
}

func (h *handler) Execute(inputs Inputs) error {
	workflowDir, err := filepath.Abs(inputs.WorkflowFolder)
	if err != nil {
		return fmt.Errorf("workflow folder path: %w", err)
	}
	workflowYAML := filepath.Join(workflowDir, constants.DefaultWorkflowSettingsFileName)
	currentPath, err := settings.GetWorkflowPathFromFile(workflowYAML)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("workflow folder does not contain %s: %w", constants.DefaultWorkflowSettingsFileName, err)
		}
		return err
	}
	workflowPath, err := cmdcommon.ResolveWorkflowPath(workflowDir, currentPath)
	if err != nil {
		return fmt.Errorf("cannot detect workflow language: %w", err)
	}
	lang := cmdcommon.GetWorkflowLanguage(workflowPath)
	if lang == constants.WorkflowLanguageWasm {
		return fmt.Errorf("workflow is already a custom build (workflow-path is %s)", currentPath)
	}

	if !inputs.Force {
		confirmed, err := prompt.YesNoPrompt(h.stdin, convertWarning)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Convert cancelled.")
			return nil
		}
	}

	if err := settings.SetWorkflowPathInFile(workflowYAML, wasmWorkflowPath); err != nil {
		return err
	}

	wasmDir := filepath.Join(workflowDir, "wasm")
	if err := os.MkdirAll(wasmDir, 0755); err != nil {
		return fmt.Errorf("create wasm directory: %w", err)
	}

	makefilePath := filepath.Join(workflowDir, "Makefile")
	makefile, err := makefileContent(workflowDir, lang)
	if err != nil {
		return err
	}
	if err := os.WriteFile(makefilePath, []byte(makefile), 0600); err != nil {
		return fmt.Errorf("write Makefile: %w", err)
	}

	fmt.Println("Workflow converted to custom build. workflow-path is now", wasmWorkflowPath)
	fmt.Println("Implement the build target in the Makefile, then run: make build")
	return nil
}

const (
	creSdkNodeModulesPath = "node_modules/@chainlink/cre-sdk"
	creScriptsDir         = "scripts/src"
	creDotCreDir          = ".cre"
)

func goMakefile() string {
	return `.PHONY: build

build:
	GOOS=wasip1 GOARCH=wasm CGO_ENABLED=0 go build -o wasm/workflow.wasm -trimpath -ldflags="-buildid= -w -s" .
`
}

func makefileContent(workflowDir, lang string) (string, error) {
	switch lang {
	case constants.WorkflowLanguageGolang:
		return goMakefile(), nil
	case constants.WorkflowLanguageTypeScript:
		return makefileContentTS(workflowDir)
	default:
		return "", fmt.Errorf("unsupported workflow language")
	}
}

func makefileContentTS(workflowDir string) (string, error) {
	sdkRoot := filepath.Join(workflowDir, creSdkNodeModulesPath)
	compileToJS := filepath.Join(sdkRoot, creScriptsDir, "compile-to-js.ts")
	wrapper := filepath.Join(sdkRoot, creScriptsDir, "workflow-wrapper.ts")
	if _, err := os.Stat(compileToJS); err != nil {
		if os.IsNotExist(err) {
			if _, errDir := os.Stat(filepath.Join(workflowDir, "node_modules")); os.IsNotExist(errDir) {
				cmd := exec.Command("bun", "install")
				cmd.Dir = workflowDir
				if out, runErr := cmd.CombinedOutput(); runErr != nil {
					return "", fmt.Errorf("TypeScript workflow requires dependencies; run 'bun install' in the workflow directory: %w\n%s", runErr, out)
				}
			}
		}
		if _, err = os.Stat(compileToJS); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("TypeScript workflow requires %s; run 'bun install' in the workflow directory first", creSdkNodeModulesPath)
			}
			return "", fmt.Errorf("check %s: %w", compileToJS, err)
		}
	}
	if _, err := os.Stat(wrapper); err != nil {
		return "", fmt.Errorf("workflow-wrapper not found: %w", err)
	}

	dotCre := filepath.Join(workflowDir, creDotCreDir)
	if err := os.MkdirAll(dotCre, 0755); err != nil {
		return "", fmt.Errorf("create %s: %w", creDotCreDir, err)
	}

	compileToJSBytes, err := os.ReadFile(compileToJS)
	if err != nil {
		return "", err
	}
	compileToJSSrc := string(compileToJSBytes)
	compileToJSSrc = strings.Replace(compileToJSSrc, "process.argv.slice(3)", "process.argv.slice(2)", 1)
	compileToJSSrc = strings.TrimSuffix(compileToJSSrc, "\n") + "\n\nmain().catch((err: unknown) => { console.error(err); process.exit(1); })\n"
	if err := os.WriteFile(filepath.Join(dotCre, "compile-to-js.ts"), []byte(compileToJSSrc), 0600); err != nil {
		return "", err
	}

	wrapperBytes, err := os.ReadFile(wrapper)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dotCre, "workflow-wrapper.ts"), wrapperBytes, 0600); err != nil {
		return "", err
	}

	return `.PHONY: build

build:
	bun .cre/compile-to-js.ts main.ts $(CURDIR)/wasm/workflow.js
	bunx cre-compile-workflow $(CURDIR)/wasm/workflow.js $(CURDIR)/wasm/workflow.wasm
`, nil
}

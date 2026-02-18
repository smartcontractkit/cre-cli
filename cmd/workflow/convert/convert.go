package convert

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const (
	wasmWorkflowPath = "./wasm/workflow.wasm"
	convertWarning   = "This will convert your workflow to a custom (self-compiled) build. This cannot be undone by the CLI. Continue?"
)

type Inputs struct {
	WorkflowFolder string
	Force          bool
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var force bool
	convertCmd := &cobra.Command{
		Use:     "custom-build <workflow-folder-path>",
		Short:   "Converts an existing workflow to a custom (self-compiled) build",
		Long:    `Converts a Go or TypeScript workflow to use a custom build via Makefile, producing wasm/workflow.wasm. The workflow-path in workflow.yaml is updated to ./wasm/workflow.wasm. This cannot be undone.`,
		Args:    cobra.ExactArgs(1),
		Example: `cre workflow custom-build ./my-workflow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext)
			inputs := Inputs{
				WorkflowFolder: args[0],
				Force:          force,
			}
			return handler.Execute(inputs)
		},
	}
	convertCmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt and convert immediately")
	return convertCmd
}

// confirmFn is the type for the confirmation prompt; production uses ui.Confirm (Charm).
type confirmFn func(title string, opts ...ui.ConfirmOption) (bool, error)

type handler struct {
	log            *zerolog.Logger
	runtimeContext *runtime.Context
	confirmFn      confirmFn // always set: ui.Confirm in production, test double in tests
}

func newHandler(runtimeContext *runtime.Context) *handler {
	h := &handler{runtimeContext: runtimeContext, confirmFn: ui.Confirm}
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
		confirmed, err := h.confirmFn(convertWarning, ui.WithLabels("Yes", "No"))
		if err != nil {
			return err
		}
		if !confirmed {
			ui.Dim("Convert cancelled.")
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
	mainFile := filepath.Base(workflowPath)
	makefile, err := makefileContent(workflowDir, lang, mainFile)
	if err != nil {
		return err
	}
	if err := os.WriteFile(makefilePath, []byte(makefile), 0600); err != nil {
		return fmt.Errorf("write Makefile: %w", err)
	}

	ui.Success("Workflow converted to custom build. workflow-path is now " + wasmWorkflowPath)
	ui.Dim("The Makefile is configured to output the WASM to this path. Run: make build")
	return nil
}

func goMakefile() string {
	return `.PHONY: build

export GOOS := wasip1
export GOARCH := wasm
export CGO_ENABLED := 0

build:
	go build -o wasm/workflow.wasm -trimpath -ldflags="-buildid= -w -s" .
`
}

func makefileContent(workflowDir, lang string, mainFile string) (string, error) {
	switch lang {
	case constants.WorkflowLanguageGolang:
		return goMakefile(), nil
	case constants.WorkflowLanguageTypeScript:
		return makefileContentTS(workflowDir, mainFile)
	default:
		return "", fmt.Errorf("unsupported workflow language")
	}
}

func makefileContentTS(_, mainFile string) (string, error) {
	return fmt.Sprintf(`.PHONY: build

build:
	bun cre-compile %s wasm/workflow.wasm
`, mainFile), nil
}

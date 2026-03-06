package build

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

var defaultOutputPath = "./binary.wasm"

func New(runtimeContext *runtime.Context) *cobra.Command {
	buildCmd := &cobra.Command{
		Use:     "build <workflow-folder-path>",
		Short:   "Compiles a workflow to a WASM binary",
		Long:    `Compiles the workflow to WASM and writes the raw binary to a file. Does not upload, register, or simulate.`,
		Args:    cobra.ExactArgs(1),
		Example: `cre workflow build ./my-workflow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputPath, _ := cmd.Flags().GetString("output")
			return execute(outputPath)
		},
	}
	buildCmd.Flags().StringP("output", "o", defaultOutputPath, "Output file path for the compiled WASM binary")
	return buildCmd
}

func execute(outputPath string) error {
	workflowDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("workflow directory: %w", err)
	}

	workflowYAML := filepath.Join(workflowDir, constants.DefaultWorkflowSettingsFileName)
	pathFromYAML, err := settings.GetWorkflowPathFromFile(workflowYAML)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("workflow folder does not contain %s: %w", constants.DefaultWorkflowSettingsFileName, err)
		}
		return fmt.Errorf("read workflow settings: %w", err)
	}

	resolvedPath, err := cmdcommon.ResolveWorkflowPath(workflowDir, pathFromYAML)
	if err != nil {
		return fmt.Errorf("resolve workflow path: %w", err)
	}

	outputPath = cmdcommon.EnsureWasmExtension(outputPath)

	ui.Dim("Compiling workflow...")
	wasmBytes, err := cmdcommon.CompileWorkflowToWasm(resolvedPath)
	if err != nil {
		ui.Error("Build failed:")
		return fmt.Errorf("failed to compile workflow: %w", err)
	}
	ui.Success("Workflow compiled successfully")

	if err := os.WriteFile(outputPath, wasmBytes, 0666); err != nil { //nolint:gosec
		return fmt.Errorf("failed to write WASM binary: %w", err)
	}

	ui.Success(fmt.Sprintf("Build output written to %s", outputPath))
	return nil
}

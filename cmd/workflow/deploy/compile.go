package deploy

import (
	"fmt"
	"os"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

func (h *handler) Compile() error {
	if !h.validated {
		return fmt.Errorf("handler h.inputs not validated")
	}

	// URL wasm is handled directly in Execute(); nothing to compile or write locally.
	if cmdcommon.IsURL(h.inputs.WasmPath) {
		return nil
	}

	if h.inputs.OutputPath == "" {
		h.inputs.OutputPath = defaultOutputPath
	}
	h.inputs.OutputPath = cmdcommon.EnsureOutputExtension(h.inputs.OutputPath)

	var wasmFile []byte
	var err error

	if h.inputs.WasmPath != "" {
		ui.Dim("Reading pre-built WASM binary...")
		wasmFile, err = os.ReadFile(h.inputs.WasmPath)
		if err != nil {
			return fmt.Errorf("failed to read WASM binary from %s: %w", h.inputs.WasmPath, err)
		}
		if h.runtimeContext != nil {
			h.runtimeContext.Workflow.Language = constants.WorkflowLanguageWasm
		}
		h.log.Debug().Str("path", h.inputs.WasmPath).Msg("Loaded pre-built WASM binary")

		br64Data, err := cmdcommon.EnsureBrotliBase64(wasmFile)
		if err != nil {
			return fmt.Errorf("failed to process WASM binary: %w", err)
		}
		if err = os.WriteFile(h.inputs.OutputPath, br64Data, 0666); err != nil { //nolint:gosec
			return fmt.Errorf("failed to write output: %w", err)
		}
		ui.Success(fmt.Sprintf("Loaded pre-built WASM binary from %s", h.inputs.WasmPath))
		return nil
	}

	ui.Dim("Compiling workflow...")

	workflowDir, dirErr := os.Getwd()
	if dirErr != nil {
		return fmt.Errorf("workflow directory: %w", dirErr)
	}
	resolvedWorkflowPath, resolveErr := cmdcommon.ResolveWorkflowPath(workflowDir, h.inputs.WorkflowPath)
	if resolveErr != nil {
		return fmt.Errorf("workflow path: %w", resolveErr)
	}
	_, workflowMainFile, mainErr := cmdcommon.WorkflowPathRootAndMain(resolvedWorkflowPath)
	if mainErr != nil {
		return fmt.Errorf("workflow path: %w", mainErr)
	}
	if h.runtimeContext != nil {
		h.runtimeContext.Workflow.Language = cmdcommon.GetWorkflowLanguage(workflowMainFile)
	}

	wasmFile, err = cmdcommon.CompileWorkflowToWasm(resolvedWorkflowPath, cmdcommon.WorkflowCompileOptions{
		StripSymbols:   true,
		SkipTypeChecks: h.inputs.SkipTypeChecks,
	})
	if err != nil {
		ui.Error("Build failed:")
		return fmt.Errorf("failed to compile workflow: %w", err)
	}
	h.log.Debug().Msg("Workflow compiled successfully")
	ui.Success("Workflow compiled successfully")

	compressedFile, err := cmdcommon.CompressBrotli(wasmFile)
	if err != nil {
		return fmt.Errorf("failed to compress WASM binary: %w", err)
	}
	h.log.Debug().Msg("WASM binary compressed")

	if err = cmdcommon.EncodeBase64ToFile(compressedFile, h.inputs.OutputPath); err != nil {
		return fmt.Errorf("failed to base64 encode the WASM binary: %w", err)
	}
	h.log.Debug().Msg("WASM binary encoded")

	return nil
}

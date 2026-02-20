package deploy

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/andybalholm/brotli"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

func (h *handler) Compile() error {
	if !h.validated {
		return fmt.Errorf("handler h.inputs not validated")
	}
	ui.Dim("Compiling workflow...")

	if h.inputs.OutputPath == "" {
		h.inputs.OutputPath = defaultOutputPath
	}
	if !strings.HasSuffix(h.inputs.OutputPath, ".b64") {
		if !strings.HasSuffix(h.inputs.OutputPath, ".br") {
			if !strings.HasSuffix(h.inputs.OutputPath, ".wasm") {
				h.inputs.OutputPath += ".wasm" // Append ".wasm" if it doesn't already end with ".wasm"
			}
			h.inputs.OutputPath += ".br" // Append ".br" if it doesn't already end with ".br"
		}
		h.inputs.OutputPath += ".b64" // Append ".b64" if it doesn't already end with ".b64"
	}

	workflowDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("workflow directory: %w", err)
	}
	resolvedWorkflowPath, err := cmdcommon.ResolveWorkflowPath(workflowDir, h.inputs.WorkflowPath)
	if err != nil {
		return fmt.Errorf("workflow path: %w", err)
	}
	_, workflowMainFile, err := cmdcommon.WorkflowPathRootAndMain(resolvedWorkflowPath)
	if err != nil {
		return fmt.Errorf("workflow path: %w", err)
	}
	if h.runtimeContext != nil {
		h.runtimeContext.Workflow.Language = cmdcommon.GetWorkflowLanguage(workflowMainFile)
	}

	wasmFile, err := cmdcommon.CompileWorkflowToWasm(resolvedWorkflowPath)
	if err != nil {
		ui.Error("Build failed:")
		return fmt.Errorf("failed to compile workflow: %w", err)
	}
	h.log.Debug().Msg("Workflow compiled successfully")
	ui.Success("Workflow compiled successfully")

	compressedFile, err := applyBrotliCompressionV2(&wasmFile)
	if err != nil {
		return fmt.Errorf("failed to compress WASM binary: %w", err)
	}
	h.log.Debug().Msg("WASM binary compressed")

	if err = encodeToBase64AndSaveToFile(&compressedFile, h.inputs.OutputPath); err != nil {
		return fmt.Errorf("failed to base64 encode the WASM binary: %w", err)
	}
	h.log.Debug().Msg("WASM binary encoded")

	return nil
}

func applyBrotliCompressionV2(wasmContent *[]byte) ([]byte, error) {
	var buffer bytes.Buffer

	// Compress using Brotli with default options
	writer := brotli.NewWriter(&buffer)

	_, err := writer.Write(*wasmContent)
	if err != nil {
		return nil, err
	}

	// must close it to flush the writer and ensure all data is stored to the buffer
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func encodeToBase64AndSaveToFile(input *[]byte, outputFile string) error {
	encoded := base64.StdEncoding.EncodeToString(*input)

	err := os.WriteFile(outputFile, []byte(encoded), 0666) //nolint:gosec
	if err != nil {
		return err
	}

	return nil
}

package deploy

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andybalholm/brotli"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
)

func (h *handler) Compile() error {
	if !h.validated {
		return fmt.Errorf("handler h.inputs not validated")
	}
	fmt.Println("Compiling workflow...")

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

	workflowAbsFile, err := filepath.Abs(h.inputs.WorkflowPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for the workflow file: %w", err)
	}

	if _, err := os.Stat(workflowAbsFile); os.IsNotExist(err) {
		return fmt.Errorf("workflow file not found: %s", workflowAbsFile)
	}

	workflowRootFolder := filepath.Dir(h.inputs.WorkflowPath)

	tmpWasmFileName := "tmp.wasm"
	workflowMainFile := filepath.Base(h.inputs.WorkflowPath)

	// Set language in runtime context based on workflow file extension
	if h.runtimeContext != nil {
		h.runtimeContext.Workflow.Language = cmdcommon.GetWorkflowLanguage(workflowMainFile)

		switch h.runtimeContext.Workflow.Language {
		case constants.WorkflowLanguageTypeScript:
			if err := cmdcommon.EnsureTool("bun"); err != nil {
				return errors.New("bun is required for TypeScript workflows but was not found in PATH; install from https://bun.com/docs/installation")
			}
		case constants.WorkflowLanguageGolang:
			if err := cmdcommon.EnsureTool("go"); err != nil {
				return errors.New("go toolchain is required for Go workflows but was not found in PATH; install from https://go.dev/dl")
			}
		default:
			return fmt.Errorf("unsupported workflow language for file %s", workflowMainFile)
		}
	}

	buildCmd := cmdcommon.GetBuildCmd(workflowMainFile, tmpWasmFileName, workflowRootFolder)
	h.log.Debug().
		Str("Workflow directory", buildCmd.Dir).
		Str("Command", buildCmd.String()).
		Msg("Executing go build command")

	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(buildOutput))
		return fmt.Errorf("failed to compile workflow: %w", err)
	}
	h.log.Debug().Msgf("Build output: %s", buildOutput)
	fmt.Println("Workflow compiled successfully")

	tmpWasmLocation := filepath.Join(workflowRootFolder, tmpWasmFileName)
	wasmFile, err := os.ReadFile(tmpWasmLocation)
	if err != nil {
		return fmt.Errorf("failed to read workflow binary: %w", err)
	}

	compressedFile, err := applyBrotliCompressionV2(&wasmFile)
	if err != nil {
		return fmt.Errorf("failed to compress WASM binary: %w", err)
	}
	h.log.Debug().Msg("WASM binary compressed")

	if err = encodeToBase64AndSaveToFile(&compressedFile, h.inputs.OutputPath); err != nil {
		return fmt.Errorf("failed to base64 encode the WASM binary: %w", err)
	}
	h.log.Debug().Msg("WASM binary encoded")

	if err = os.Remove(tmpWasmLocation); err != nil {
		return fmt.Errorf("failed to remove the temporary file:  %w", err)
	}

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

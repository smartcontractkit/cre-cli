package deploy

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/andybalholm/brotli"
)

func (h *handler) Compile() error {
	if !h.validated {
		return fmt.Errorf("handler h.inputs not validated")
	}

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

	// The build command for reproducible and trimmed binaries.
	// -trimpath removes all file system paths from the compiled binary.
	// -ldflags="-buildid= -w -s" further reduces the binary size:
	//   -buildid= removes the build ID, ensuring reproducibility.
	//   -w disables DWARF debugging information.
	//   -s removes the symbol table.
	buildCmd := exec.Command(
		"go",
		"build",
		"-o", tmpWasmFileName,
		"-trimpath",
		"-ldflags=-buildid= -w -s",
		workflowMainFile,
	)

	buildCmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
	buildCmd.Dir = workflowRootFolder
	h.log.Debug().
		Str("Workflow directory", buildCmd.Dir).
		Str("Command", buildCmd.String()).
		Msg("Executing go build command")

	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		h.log.Info().Msg(string(buildOutput))
		return fmt.Errorf("failed to compile workflow: %w", err)
	}
	h.log.Debug().Msgf("Build output: %s", buildOutput)
	h.log.Info().Msg("Workflow compiled")

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

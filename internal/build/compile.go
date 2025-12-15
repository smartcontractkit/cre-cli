package build

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/rs/zerolog"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
)

type Builder struct {
	log *zerolog.Logger
}

func NewBuilder(log *zerolog.Logger) *Builder {
	return &Builder{
		log: log,
	}
}

func (b *Builder) Compile(workflowPath string) (*[]byte, error) {
	fmt.Println("Compiling workflow...")

	workflowAbsFile, err := filepath.Abs(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for the workflow file: %w", err)
	}

	if _, err := os.Stat(workflowAbsFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("workflow file not found: %s", workflowAbsFile)
	}

	workflowRootFolder := filepath.Dir(workflowPath)

	tmpWasmFileName := "tmp.wasm"
	workflowMainFile := filepath.Base(workflowPath)

	switch cmdcommon.GetWorkflowLanguage(workflowMainFile) {
	case constants.WorkflowLanguageTypeScript:
		if err := cmdcommon.EnsureTool("bun"); err != nil {
			return nil, errors.New("bun is required for TypeScript workflows but was not found in PATH; install from https://bun.com/docs/installation")
		}
	case constants.WorkflowLanguageGolang:
		if err := cmdcommon.EnsureTool("go"); err != nil {
			return nil, errors.New("go toolchain is required for Go workflows but was not found in PATH; install from https://go.dev/dl")
		}
	default:
		return nil, fmt.Errorf("unsupported workflow language for file %s", workflowMainFile)
	}

	buildCmd := cmdcommon.GetBuildCmd(workflowMainFile, tmpWasmFileName, workflowRootFolder)
	b.log.Debug().
		Str("Workflow directory", buildCmd.Dir).
		Str("Command", buildCmd.String()).
		Msg("Executing go build command")

	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(buildOutput))

		out := strings.TrimSpace(string(buildOutput))
		return nil, fmt.Errorf("failed to compile workflow: %w\nbuild output:\n%s", err, out)
	}
	b.log.Debug().Msgf("Build output: %s", buildOutput)
	fmt.Println("Workflow compiled successfully")

	tmpWasmLocation := filepath.Join(workflowRootFolder, tmpWasmFileName)
	wasmFile, err := os.ReadFile(tmpWasmLocation)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow binary: %w", err)
	}

	compressedFile, err := applyBrotliCompressionV2(&wasmFile)
	if err != nil {
		return nil, fmt.Errorf("failed to compress WASM binary: %w", err)
	}
	b.log.Debug().Msg("WASM binary compressed")

	encoded := encodeToBase64(&compressedFile)
	b.log.Debug().Msg("WASM binary encoded")

	if err = os.Remove(tmpWasmLocation); err != nil {
		return nil, fmt.Errorf("failed to remove the temporary file:  %w", err)
	}

	return encoded, nil
}

func (b *Builder) CompileAndSave(workflowPath, outputPath string) error {
	if outputPath == "" {
		return fmt.Errorf("output path is not specified")
	}
	outputPath = ensureOutputPathExtensions(outputPath)

	binary, err := b.Compile(workflowPath)
	if err != nil {
		return err
	}

	return saveToFile(binary, outputPath)
}

func ensureOutputPathExtensions(outputPath string) string {
	if !strings.HasSuffix(outputPath, ".b64") {
		if !strings.HasSuffix(outputPath, ".br") {
			if !strings.HasSuffix(outputPath, ".wasm") {
				outputPath += ".wasm" // Append ".wasm" if it doesn't already end with ".wasm"
			}
			outputPath += ".br" // Append ".br" if it doesn't already end with ".br"
		}
		outputPath += ".b64" // Append ".b64" if it doesn't already end with ".b64"
	}
	return outputPath
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

func encodeToBase64(input *[]byte) *[]byte {
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(*input)))
	base64.StdEncoding.Encode(encoded, *input)
	return &encoded
}

func saveToFile(input *[]byte, outputFile string) error {
	err := os.WriteFile(outputFile, *input, 0666) //nolint:gosec
	if err != nil {
		return err
	}
	return nil
}

package build_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/build"
)

func TestResolveBuildParamsForWorkflow(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		workflowFile := filepath.Join(tempDir, "main.go")
		err := os.WriteFile(workflowFile, []byte("package main"), 0600)
		require.NoError(t, err)

		tests := []struct {
			name               string
			workflowPath       string
			outputPath         string
			expectedMainFile   string
			expectedRootFolder string
			expectedLanguage   string
		}{
			{
				name:               "go workflow",
				workflowPath:       workflowFile,
				outputPath:         "output.wasm",
				expectedMainFile:   "main.go",
				expectedRootFolder: tempDir,
				expectedLanguage:   "golang",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				params, err := build.ResolveBuildParamsForWorkflow(tt.workflowPath, tt.outputPath)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedMainFile, params.WorkflowMainFile)
				assert.Equal(t, tt.expectedRootFolder, params.WorkflowRootFolder)
				assert.Equal(t, tt.expectedLanguage, params.WorkflowLanguage)
				assert.Equal(t, tt.outputPath, params.OutputPath)
				assert.Contains(t, params.WorkflowPath, tt.expectedMainFile)
			})
		}
	})

	t.Run("typescript workflow", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		workflowFile := filepath.Join(tempDir, "main.ts")
		err := os.WriteFile(workflowFile, []byte("console.log('test')"), 0600)
		require.NoError(t, err)

		params, err := build.ResolveBuildParamsForWorkflow(workflowFile, "output.wasm")
		require.NoError(t, err)
		assert.Equal(t, "main.ts", params.WorkflowMainFile)
		assert.Equal(t, tempDir, params.WorkflowRootFolder)
		assert.Equal(t, "typescript", params.WorkflowLanguage)
	})

	t.Run("errors", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name         string
			workflowPath string
			wantErr      string
		}{
			{
				name:         "workflow file not found",
				workflowPath: "/nonexistent/path/main.go",
				wantErr:      "workflow file not found",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := build.ResolveBuildParamsForWorkflow(tt.workflowPath, "output.wasm")
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			})
		}
	})
}

func TestBuilderCompileAndSave(t *testing.T) {
	t.Parallel()

	logger := zerolog.New(os.Stdout)
	builder := build.NewBuilder(&logger)

	t.Run("error when output path is empty", func(t *testing.T) {
		t.Parallel()

		params := build.Params{
			OutputPath: "",
		}

		err := builder.CompileAndSave(params)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output path is not specified")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		workflowPath := filepath.Join("testdata", "basic_workflow", "main.go")
		outputPath := "./binary.wasm.br.b64"
		defer os.Remove(outputPath)

		params, err := build.ResolveBuildParamsForWorkflow(workflowPath, outputPath)
		require.NoError(t, err)

		err = builder.CompileAndSave(params)
		require.NoError(t, err)

		assert.FileExists(t, outputPath)
	})
}

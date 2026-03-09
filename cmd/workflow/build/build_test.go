package build

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
)

func TestEnsureWasmExtension(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no extension", "./my-binary", "./my-binary.wasm"},
		{"already .wasm", "./my-binary.wasm", "./my-binary.wasm"},
		{"default path", "./binary.wasm", "./binary.wasm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, cmdcommon.EnsureWasmExtension(tt.input))
		})
	}
}

func TestBuildCommandArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "no args provided",
			args:    []string{},
			wantErr: "accepts 1 arg(s), received 0",
		},
		{
			name:    "too many args",
			args:    []string{"path1", "path2"},
			wantErr: "accepts 1 arg(s), received 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := New(nil)
			cmd.SetArgs(tt.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			err := cmd.Execute()
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestBuildCommandDefaultFlag(t *testing.T) {
	t.Parallel()
	cmd := New(nil)
	f := cmd.Flags().Lookup("output")
	require.NotNil(t, f)
	assert.Equal(t, "", f.DefValue)
	assert.Equal(t, "o", f.Shorthand)
}

func TestBuildMissingWorkflowYAML(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	cmd := New(nil)
	cmd.SetArgs([]string{tmpDir})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "workflow.yaml")
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	require.NoError(t, err)
	for _, entry := range entries {
		name := filepath.Base(entry.Name())
		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)
		if entry.IsDir() {
			require.NoError(t, os.MkdirAll(dstPath, 0755))
			copyDir(t, srcPath, dstPath)
		} else {
			data, err := os.ReadFile(srcPath)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(dstPath, data, 0600))
		}
	}
}

func setupWorkflowDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	copyDir(t, filepath.Join("..", "deploy", "testdata", "basic_workflow"), tmpDir)
	workflowYAML := `staging-settings:
  workflow-artifacts:
    workflow-path: main.go
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowYAML), 0600))
	return tmpDir
}

func TestBuildHappyPath(t *testing.T) {
	workflowDir := setupWorkflowDir(t)
	outputPath := filepath.Join(t.TempDir(), "output.wasm")

	cmd := New(nil)
	cmd.SetArgs([]string{workflowDir, "-o", outputPath})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	assert.True(t, cmdcommon.IsRawWasm(data), "output should be raw WASM (starts with \\0asm magic)")
}

func TestBuildHappyPathDefaultOutput(t *testing.T) {
	workflowDir := setupWorkflowDir(t)

	cmd := New(nil)
	cmd.SetArgs([]string{workflowDir})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.NoError(t, err)

	expectedFile := filepath.Join(workflowDir, "binary.wasm")

	data, err := os.ReadFile(expectedFile)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	assert.True(t, cmdcommon.IsRawWasm(data), "output should be raw WASM (starts with \\0asm magic)")
}

func TestBuildCustomOutputPath(t *testing.T) {
	workflowDir := setupWorkflowDir(t)
	outputPath := filepath.Join(t.TempDir(), "custom")

	cmd := New(nil)
	cmd.SetArgs([]string{workflowDir, "-o", outputPath})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.NoError(t, err)

	extendedPath := outputPath + ".wasm"
	data, err := os.ReadFile(extendedPath)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	assert.True(t, cmdcommon.IsRawWasm(data), "output should be raw WASM")
}

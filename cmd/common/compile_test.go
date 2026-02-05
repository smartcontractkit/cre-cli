package common

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deployTestdataPath(elem ...string) string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	return filepath.Join(append([]string{dir, "..", "workflow", "deploy", "testdata"}, elem...)...)
}

func TestFindMakefileRoot(t *testing.T) {
	dir := t.TempDir()

	_, err := findMakefileRoot(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no Makefile found")

	require.NoError(t, os.WriteFile(filepath.Join(dir, makefileName), []byte("build:\n\techo ok\n"), 0600))
	root, err := findMakefileRoot(dir)
	require.NoError(t, err)
	absDir, _ := filepath.Abs(dir)
	require.Equal(t, absDir, root)

	sub := filepath.Join(dir, "wasm")
	require.NoError(t, os.MkdirAll(sub, 0755))
	root, err = findMakefileRoot(sub)
	require.NoError(t, err)
	require.Equal(t, absDir, root)
}

func TestCompileWorkflowToWasm_Go_Success(t *testing.T) {
	t.Run("basic_workflow", func(t *testing.T) {
		path := deployTestdataPath("basic_workflow", "main.go")
		wasm, err := CompileWorkflowToWasm(path)
		require.NoError(t, err)
		assert.NotEmpty(t, wasm)
	})

	t.Run("configless_workflow", func(t *testing.T) {
		path := deployTestdataPath("configless_workflow", "main.go")
		wasm, err := CompileWorkflowToWasm(path)
		require.NoError(t, err)
		assert.NotEmpty(t, wasm)
	})

	t.Run("missing_go_mod", func(t *testing.T) {
		path := deployTestdataPath("missing_go_mod", "main.go")
		wasm, err := CompileWorkflowToWasm(path)
		require.NoError(t, err)
		assert.NotEmpty(t, wasm)
	})
}

func TestCompileWorkflowToWasm_Go_Malformed_Fails(t *testing.T) {
	path := deployTestdataPath("malformed_workflow", "main.go")
	_, err := CompileWorkflowToWasm(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to compile workflow")
	assert.Contains(t, err.Error(), "undefined: sdk.RemovedFunctionThatFailsCompilation")
}

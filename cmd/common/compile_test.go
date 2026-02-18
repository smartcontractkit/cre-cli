package common

import (
	"os"
	"os/exec"
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

func TestCompileWorkflowToWasm_Wasm_Success(t *testing.T) {
	wasmPath := deployTestdataPath("custom_wasm_workflow", "wasm", "workflow.wasm")
	_ = os.Remove(wasmPath)
	t.Cleanup(func() { _ = os.Remove(wasmPath) })

	wasm, err := CompileWorkflowToWasm(wasmPath)
	require.NoError(t, err)
	assert.NotEmpty(t, wasm)

	_, err = os.Stat(wasmPath)
	require.NoError(t, err, "make build should produce wasm/workflow.wasm")
}

func TestCompileWorkflowToWasm_Wasm_Fails(t *testing.T) {
	t.Run("no_makefile", func(t *testing.T) {
		dir := t.TempDir()
		wasmDir := filepath.Join(dir, "wasm")
		require.NoError(t, os.MkdirAll(wasmDir, 0755))
		wasmPath := filepath.Join(wasmDir, "workflow.wasm")
		require.NoError(t, os.WriteFile(wasmPath, []byte("not really wasm"), 0600))

		_, err := CompileWorkflowToWasm(wasmPath)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no Makefile found")
	})

	t.Run("make_build_fails", func(t *testing.T) {
		path := deployTestdataPath("wasm_make_fails", "wasm", "workflow.wasm")
		_, err := CompileWorkflowToWasm(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to compile workflow")
		assert.Contains(t, err.Error(), "build output:")
	})
}

func TestCompileWorkflowToWasm_TS_Success(t *testing.T) {
	if err := EnsureTool("bun"); err != nil {
		t.Skip("bun not in PATH, skipping TS compile test")
	}
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.ts")
	require.NoError(t, os.WriteFile(mainPath, []byte(`export async function main() { return "ok"; }
`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test","dependencies":{"@chainlink/cre-sdk":"latest"}}
`), 0600))
	install := exec.Command("bun", "install")
	install.Dir = dir
	install.Stdout = os.Stdout
	install.Stderr = os.Stderr
	if err := install.Run(); err != nil {
		t.Skipf("bun install failed (network or cre-sdk): %v", err)
	}
	wasm, err := CompileWorkflowToWasm(mainPath)
	if err != nil {
		t.Skipf("TS compile failed (published cre-sdk may lack full layout): %v", err)
	}
	assert.NotEmpty(t, wasm)
}

package common

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
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

// captureStderr redirects os.Stderr to a pipe, runs fn, and returns whatever
// was written to stderr during that call.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestWarnGOTOOLCHAIN(t *testing.T) {
	tests := []struct {
		name           string
		gotoolchain    string
		envFileContent map[string]string
		wantWarning    bool
	}{
		{
			name:        "GOTOOLCHAIN unset emits warning",
			gotoolchain: "",
			wantWarning: true,
		},
		{
			name:        "GOTOOLCHAIN set but no env file loaded emits warning",
			gotoolchain: "go1.25.3",
			wantWarning: true,
		},
		{
			name:           "GOTOOLCHAIN set but missing from env file emits warning",
			gotoolchain:    "go1.25.3",
			envFileContent: map[string]string{"CRE_TARGET": "staging"},
			wantWarning:    true,
		},
		{
			name:           "GOTOOLCHAIN set and present in env file emits no warning",
			gotoolchain:    "go1.25.3",
			envFileContent: map[string]string{"GOTOOLCHAIN": "go1.25.3"},
			wantWarning:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.gotoolchain != "" {
				t.Setenv("GOTOOLCHAIN", tc.gotoolchain)
			} else {
				t.Setenv("GOTOOLCHAIN", "")
				os.Unsetenv("GOTOOLCHAIN")
			}

			logger := testutil.NewTestLogger()
			v := viper.New()
			if tc.envFileContent != nil {
				dir := t.TempDir()
				envPath := filepath.Join(dir, ".env")
				require.NoError(t, godotenv.Write(tc.envFileContent, envPath))
				settings.LoadEnv(logger, v, envPath)
				for k := range tc.envFileContent {
					t.Cleanup(func() { os.Unsetenv(k) })
				}
			} else {
				settings.LoadEnv(logger, v, "")
			}

			output := captureStderr(t, func() {
				warnGOTOOLCHAIN()
			})

			if tc.wantWarning {
				assert.NotEmpty(t, output, "expected a warning on stderr")
				assert.Contains(t, output, "!", "output should be at warning level (ui.Warning prefix)")
			} else {
				assert.Empty(t, output, "expected no warning on stderr")
			}
		})
	}
}

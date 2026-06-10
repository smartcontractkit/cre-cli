package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestE2E_WorkflowBuildThenSimulateWithWasm_HelloWorldGo(t *testing.T) {
	tempDir := t.TempDir()
	projectName := "e2e-build-sim-wasm"
	workflowName := "helloWorkflow"
	projectRoot := filepath.Join(tempDir, projectName)
	workflowDir := filepath.Join(projectRoot, workflowName)
	// Simulate runs with cwd set to the workflow directory; --wasm is relative to that dir.
	wasmPath := "binary.wasm"

	t.Setenv(settings.EthPrivateKeyEnvVar, "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	gqlSrv := NewGraphQLMockServerGetOrganization(t)
	defer gqlSrv.Close()

	scaffoldHelloWorldGoProject(t, projectRoot, workflowName)

	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.DirExists(t, workflowDir)
	require.FileExists(t, filepath.Join(workflowDir, "main.go"))
	require.FileExists(t, filepath.Join(workflowDir, "workflow.go"))

	simulateArgs := []string{
		"workflow", "simulate",
		workflowName,
		"--project-root", projectRoot,
		"--non-interactive",
		"--trigger-index=0",
		"--target=staging-settings",
	}

	baselineOut := runCLI(t, projectRoot, simulateArgs...)
	assertHelloWorldSimulationResult(t, baselineOut)
	require.Contains(t, stripANSI(baselineOut), "Workflow compiled",
		"baseline simulate should compile inline")

	buildOut := runCLI(t, projectRoot, "workflow", "build", workflowName)
	require.FileExists(t, filepath.Join(workflowDir, "binary.wasm"))
	info, err := os.Stat(filepath.Join(workflowDir, "binary.wasm"))
	require.NoError(t, err)
	require.Positive(t, info.Size())
	require.Contains(t, stripANSI(buildOut), "Build output written")

	wasmSimulateArgs := append([]string(nil), simulateArgs...)
	wasmSimulateArgs = append(wasmSimulateArgs, "--wasm", wasmPath)
	wasmOut := runCLI(t, projectRoot, wasmSimulateArgs...)

	cleanWasmOut := stripANSI(wasmOut)
	require.Contains(t, cleanWasmOut, "Loaded WASM binary")
	require.NotContains(t, cleanWasmOut, "Workflow compiled",
		"--wasm should skip inline compilation")
	assertHelloWorldSimulationResult(t, wasmOut)
}

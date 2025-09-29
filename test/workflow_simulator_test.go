package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
)

// Simulates a workflow
func (tc *TestConfig) workflowSimulate(t *testing.T) string {
	t.Helper()

	// Resolve the workflow module dir: ./test_project/chainreader_workflow
	_, thisFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(thisFile)
	wfDir := filepath.Join(testDir, "test_project", "chainreader_workflow")

	// Artifact path (the CLI default)
	artifactPath := filepath.Join(wfDir, "binary.wasm.br.b64")

	// Ensure a clean slate and schedule cleanup
	_ = os.Remove(artifactPath) // ignore if it doesn't exist
	t.Cleanup(func() { _ = os.Remove(artifactPath) })

	// Build CLI args
	// Example: cre workflow simulate --target local-simulation --config config.json main.go --non-interactive --trigger-index 0
	args := []string{
		"workflow", "simulate",
		"main.go",
		"--config=config.json",
		tc.GetCliEnvFlag(),
		tc.GetCliSettingsFlag(),
		"--non-interactive",
		"--trigger-index=0",
	}

	cmd := exec.Command(CLIPath, args...)
	cmd.Dir = wfDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow simulation failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	out := stripANSI(stdout.String() + stderr.String())

	return out
}

func TestCLIWorkflowSimulator(t *testing.T) {
	// Start Anvil with pre-baked state
	anvilProc, testEthUrl := initTestEnv(t, "anvil-state-simulator.json")
	defer StopAnvil(anvilProc)

	tc := NewTestConfig(t)

	// Use linked Address3 + its key
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3), "failed to create env file")
	require.NoError(t, createCliSettingsFile(tc, constants.TestAddress3, "workflow-name", testEthUrl), "failed to create cre config file")
	require.NoError(t, createBlankProjectSettingFile(tc.ProjectDirectory+"project.yaml"), "failed to create project.yaml")
	t.Cleanup(tc.Cleanup(t))

	// Simulate with workflow using chain reader
	out := tc.workflowSimulate(t)
	require.Contains(t, out, "Workflow compiled", "expected workflow to compile.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Simulator Initialized", "expected workflow to initialize.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Getting native balances", "expected workflow to read from balance reader.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow Simulation Result", "expected simulation success.\nCLI OUTPUT:\n%s", out)
}

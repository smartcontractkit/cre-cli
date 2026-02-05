package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func convertSimulateCaptureOutput(t *testing.T, projectRoot, workflowName string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(CLIPath, "workflow", "simulate", workflowName,
		"--project-root", projectRoot,
		"--non-interactive", "--trigger-index=0",
	)
	cmd.Dir = projectRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(),
		"simulate (before convert) failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(), stderr.String())
	return stdout.String()
}

func convertSimulateRequireOutputContains(t *testing.T, projectRoot, workflowName, expectedSubstring string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(CLIPath, "workflow", "simulate", workflowName,
		"--project-root", projectRoot,
		"--non-interactive", "--trigger-index=0",
	)
	cmd.Dir = projectRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(),
		"simulate (after convert) failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(), stderr.String())
	require.Contains(t, stdout.String(), expectedSubstring,
		"simulate output after convert should contain %q", expectedSubstring)
}

// ConvertSimulateBeforeAfter runs simulate (capture output), convert, make build,
// then simulate again and verifies output contains the same expectedSubstring.
func ConvertSimulateBeforeAfter(t *testing.T, projectRoot, workflowDir, workflowName, expectedSubstring string) {
	t.Helper()
	beforeOutput := convertSimulateCaptureOutput(t, projectRoot, workflowName)
	require.Contains(t, beforeOutput, expectedSubstring,
		"baseline simulate output should contain %q", expectedSubstring)
	convertRunConvert(t, projectRoot, workflowDir)
	convertRunMakeBuild(t, workflowDir)
	convertRequireWasmExists(t, workflowDir)
	convertSimulateRequireOutputContains(t, projectRoot, workflowName, expectedSubstring)
}

func convertRunConvert(t *testing.T, projectRoot, workflowDir string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(CLIPath, "workflow", "convert-to-custom-build", workflowDir, "-f")
	cmd.Dir = projectRoot
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(),
		"convert failed:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())
}

func convertRunMakeBuild(t *testing.T, workflowDir string, env ...string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("make", "build")
	cmd.Dir = workflowDir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(),
		"make build failed:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())
}

func convertRequireWasmExists(t *testing.T, workflowDir string) {
	t.Helper()
	require.FileExists(t, filepath.Join(workflowDir, "wasm", "workflow.wasm"),
		"wasm/workflow.wasm should exist after make build")
}

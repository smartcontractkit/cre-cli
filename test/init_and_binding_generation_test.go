package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestE2EInit_DevPoRTemplate(t *testing.T) {
	tempDir := t.TempDir()
	projectName := "e2e-init-test"
	workflowName := "devPoRWorkflow"
	templateID := "1"
	projectRoot := filepath.Join(tempDir, projectName)
	workflowDirectory := filepath.Join(projectRoot, workflowName)

	ethKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	t.Setenv(settings.EthPrivateKeyEnvVar, ethKey)

	// Set dummy API key
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	initArgs := []string{
		"init",
		"--project-root", tempDir,
		"--project-name", projectName,
		"--template-id", templateID,
		"--workflow-name", workflowName,
	}
	var stdout, stderr bytes.Buffer
	initCmd := exec.Command(CLIPath, initArgs...)
	initCmd.Dir = tempDir
	initCmd.Stdout = &stdout
	initCmd.Stderr = &stderr

	require.NoError(
		t,
		initCmd.Run(),
		"cre init failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultEnvFileName))
	require.DirExists(t, workflowDirectory)

	expectedFiles := []string{"README.md", "main.go", "workflow.yaml", "workflow.go", "workflow_test.go"}
	for _, f := range expectedFiles {
		require.FileExists(t, filepath.Join(workflowDirectory, f), "missing workflow file %q", f)
	}

	// cre generate-bindings
	stdout.Reset()
	stderr.Reset()
	bindingsCmd := exec.Command(CLIPath, "generate-bindings", "evm")
	bindingsCmd.Dir = projectRoot
	bindingsCmd.Stdout = &stdout
	bindingsCmd.Stderr = &stderr

	require.NoError(
		t,
		bindingsCmd.Run(),
		"cre generate-bindings failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	// go mod tidy on project root to sync dependencies
	stdout.Reset()
	stderr.Reset()
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = projectRoot
	tidyCmd.Stdout = &stdout
	tidyCmd.Stderr = &stderr

	require.NoError(
		t,
		tidyCmd.Run(),
		"go mod tidy failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	// Check that the generated main.go file compiles successfully for WASM target
	stdout.Reset()
	stderr.Reset()
	buildCmd := exec.Command("go", "build", "-o", "workflow.wasm", ".")
	buildCmd.Dir = workflowDirectory
	buildCmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	buildCmd.Stdout = &stdout
	buildCmd.Stderr = &stderr

	require.NoError(
		t,
		buildCmd.Run(),
		"generated main.go failed to compile for WASM target:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	// Run the generated workflow tests to ensure they compile and pass
	stdout.Reset()
	stderr.Reset()
	testCmd := exec.Command("go", "test", "-v", "./...")
	testCmd.Dir = workflowDirectory
	testCmd.Stdout = &stdout
	testCmd.Stderr = &stderr

	require.NoError(
		t,
		testCmd.Run(),
		"generated workflow tests failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

}

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
	templateName := "hello-world-go" // Built-in Go template
	projectRoot := filepath.Join(tempDir, projectName)
	workflowDirectory := filepath.Join(projectRoot, workflowName)

	ethKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	t.Setenv(settings.EthPrivateKeyEnvVar, ethKey)

	// Set dummy API key
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	gqlSrv := NewGraphQLMockServerGetOrganization(t)
	defer gqlSrv.Close()

	initArgs := []string{
		"init",
		"--project-root", tempDir,
		"--project-name", projectName,
		"--template", templateName,
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

	expectedFiles := []string{"README.md", "main.go"}
	for _, f := range expectedFiles {
		require.FileExists(t, filepath.Join(workflowDirectory, f), "missing workflow file %q", f)
	}

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
	buildCmd := exec.Command("go", "build", "-o", filepath.Join(workflowDirectory, "workflow.wasm"), "./"+workflowName) //nolint:gosec // test code with controlled inputs
	buildCmd.Dir = projectRoot
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

	// --- cre workflow simulate devPoRWorkflow ---
	stdout.Reset()
	stderr.Reset()
	simulateArgs := []string{
		"workflow", "simulate",
		workflowName,
		"--project-root", projectRoot,
		"--non-interactive",
		"--trigger-index=0",
	}
	simulateCmd := exec.Command(CLIPath, simulateArgs...)
	simulateCmd.Dir = projectRoot
	simulateCmd.Stdout = &stdout
	simulateCmd.Stderr = &stderr

	require.NoError(
		t,
		simulateCmd.Run(),
		"cre workflow simulate failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)
}

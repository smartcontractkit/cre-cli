package test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
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

	initArgs := []string{
		"init",
		"--project-path", tempDir,
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

	expectedFiles := []string{"README.md", "main.go", "workflow.yaml"}
	for _, f := range expectedFiles {
		require.FileExists(t, filepath.Join(workflowDirectory, f), "missing workflow file %q", f)
	}

	// cre generate-bindings --chain-family=evm
	stdout.Reset()
	stderr.Reset()
	bindingsCmd := exec.Command(CLIPath, "generate-bindings", "--chain-family=evm")
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

	// go mod tidy on workflow directory to sync dependencies
	stdout.Reset()
	stderr.Reset()
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = workflowDirectory
	tidyCmd.Stdout = &stdout
	tidyCmd.Stderr = &stderr

	require.NoError(
		t,
		tidyCmd.Run(),
		"go mod tidy failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

}

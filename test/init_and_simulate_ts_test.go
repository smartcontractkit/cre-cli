package test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil/cretest"
)

func TestE2EInit_DevPoRTemplateTS(t *testing.T) {
	isolatedEnv(t)
	tempDir := t.TempDir()
	projectName := "e2e-init-test"
	workflowName := "devPoRWorkflow"
	templateName := "hello-world-ts" // Built-in TS template
	projectRoot := filepath.Join(tempDir, projectName)
	workflowDirectory := filepath.Join(projectRoot, workflowName)

	ethKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	t.Setenv(settings.EthPrivateKeyEnvVar, ethKey)
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
	requireCLI(t, "cre init failed", initArgs, cretest.WithDir(tempDir))

	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName))
	require.FileExists(t, filepath.Join(projectRoot, constants.DefaultEnvFileName))
	require.DirExists(t, workflowDirectory)

	expectedFiles := []string{"README.md", "main.ts", "main.test.ts", "package.json"}
	for _, f := range expectedFiles {
		require.FileExists(t, filepath.Join(workflowDirectory, f), "missing workflow file %q", f)
	}

	var stdout, stderr bytes.Buffer
	bunCmd := exec.Command("bun", "install")
	bunCmd.Dir = workflowDirectory
	bunCmd.Stdout = &stdout
	bunCmd.Stderr = &stderr
	require.NoError(t, bunCmd.Run(), "bun install failed:\nSTDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())

	simulateArgs := []string{
		"workflow", "simulate",
		workflowName,
		"--project-root", projectRoot,
		"--non-interactive",
		"--trigger-index=0",
		"--target=staging-settings",
	}
	requireCLI(t, "cre workflow simulate failed", simulateArgs, cretest.WithDir(projectRoot))
}

package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestE2EInit_DevPoRTemplateTS(t *testing.T) {
	tempDir := t.TempDir()
	projectName := "e2e-init-test"
	workflowName := "devPoRWorkflow"
	templateID := "4"
	projectRoot := filepath.Join(tempDir, projectName)
	workflowDirectory := filepath.Join(projectRoot, workflowName)

	ethKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	t.Setenv(settings.EthPrivateKeyEnvVar, ethKey)

	// Set dummy API key
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	// Set up mock GraphQL server for authentication validation
	// This is needed because validation now runs early in command execution
	gqlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost {
			var req struct {
				Query     string                 `json:"query"`
				Variables map[string]interface{} `json:"variables"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")

			// Handle authentication validation query
			if strings.Contains(req.Query, "getOrganization") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getOrganization": map[string]any{
							"organizationId": "test-org-id",
						},
					},
				})
				return
			}

			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
			})
		}
	}))
	defer gqlSrv.Close()

	// Point GraphQL client to mock server
	t.Setenv(environments.EnvVarGraphQLURL, gqlSrv.URL+"/graphql")

	initArgs := []string{
		"init",
		"--project-root", tempDir,
		"--project-name", projectName,
		"--template-id", templateID,
		"--workflow-name", workflowName,
		"--rpc-url", constants.DefaultEthSepoliaRpcUrl,
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

	expectedFiles := []string{"README.md", "main.ts", "workflow.yaml", "package.json"}
	for _, f := range expectedFiles {
		require.FileExists(t, filepath.Join(workflowDirectory, f), "missing workflow file %q", f)
	}

	// --- bun install in the workflow directory ---
	stdout.Reset()
	stderr.Reset()
	bunCmd := exec.Command("bun", "install")
	bunCmd.Dir = workflowDirectory
	bunCmd.Stdout = &stdout
	bunCmd.Stderr = &stderr

	require.NoError(
		t,
		bunCmd.Run(),
		"bun install failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
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

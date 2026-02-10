package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

	expectedFiles := []string{"README.md", "main.go", "workflow.yaml", "workflow.go", "workflow_test.go"}
	for _, f := range expectedFiles {
		require.FileExists(t, filepath.Join(workflowDirectory, f), "missing workflow file %q", f)
	}

	// cre generate-bindings
	stdout.Reset()
	stderr.Reset()
	bindingsCmd := exec.Command(CLIPath, "generate-bindings-evm")
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

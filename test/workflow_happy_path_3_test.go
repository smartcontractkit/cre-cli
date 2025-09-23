package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

// Deploys a workflow with config via CLI, using unlinked address, mocking GraphQL + Origin.
func (tc *TestConfig) workflowDeployUnlinkedWithConfig(t *testing.T, configPath string) string {
	t.Helper()

	var srv *httptest.Server
	// One server that handles both GraphQL and "origin" uploads.
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost:
			var req graphQLRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			// Respond based on the mutation in the query
			if strings.Contains(req.Query, "GeneratePresignedPostUrlForArtifact") {
				// Return presigned POST URL + fields (pointing back to this server)
				resp := map[string]any{
					"data": map[string]any{
						"generatePresignedPostUrlForArtifact": map[string]any{
							"presignedPostUrl":    srv.URL + "/upload",
							"presignedPostFields": []map[string]string{{"key": "k1", "value": "v1"}},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			if strings.Contains(req.Query, "GenerateUnsignedGetUrlForArtifact") {
				resp := map[string]any{
					"data": map[string]any{
						"generateUnsignedGetUrlForArtifact": map[string]any{
							"unsignedGetUrl": srv.URL + "/get",
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
			if strings.Contains(req.Query, "InitiateLinking") {
				// Return an error to simulate linking failure (real world scenario)
				// In reality, this would fail due to signature validation issues
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{{"message": "Invalid signature or linking request"}},
				})
				return
			}
			// Fallback error
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
			})
			return

		case r.URL.Path == "/upload" && r.Method == http.MethodPost:
			// Accept origin "upload" (presigned POST target)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("OK"))
			return

		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
			return
		}
	}))
	defer srv.Close()

	// Point the CLI at our mock GraphQL endpoint
	os.Setenv(environments.EnvVarGraphQLURL, srv.URL+"/graphql")

	// Resolve the workflow module dir: ./test_project/blank_workflow
	_, thisFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(thisFile)
	wfDir := filepath.Join(testDir, "test_project", "blank_workflow")

	// Artifact path (the CLI default)
	artifactPath := filepath.Join(wfDir, "binary.wasm.br.b64")

	// Ensure a clean slate and schedule cleanup
	_ = os.Remove(artifactPath) // ignore if it doesn't exist
	t.Cleanup(func() { _ = os.Remove(artifactPath) })

	// Build CLI args with config file
	args := []string{
		"workflow", "deploy",
		"main.go",
		tc.GetCliEnvFlag(),
		tc.GetCliSettingsFlag(),
		"--config", configPath,
	}

	cmd := exec.Command(CLIPath, args...)
	cmd.Dir = wfDir

	// Provide stdin input for auto-link prompts:
	// 1. "test-label" for owner address label prompt
	// 2. For the transaction confirmation, we need to simulate "Yes" selection
	//    On Unix (promptui): send newline to select default (first option = "Yes")
	//    On Windows (bubbletea): send newline to select default (first option = "Yes")
	stdinInput := "test-label\n\n"
	cmd.Stdin = strings.NewReader(stdinInput)

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	// The command should fail due to linking failure, but we still want to capture the output
	// to verify that it detected unlinked status and attempted to link
	err := cmd.Run()
	require.Error(t, err, "expected command to fail due to linking failure")
	// We still want to verify the output shows the linking attempt

	out := stripANSI(stdout.String() + stderr.String())

	return out
}

func TestCLIWorkflowHappyPath3DeployUnlinkedWithConfig(t *testing.T) {
	// Start Anvil with pre-baked state
	anvilProc, testEthUrl := initTestEnv(t)
	defer StopAnvil(anvilProc)

	tc := NewTestConfig(t)

	// Use TestAddress (unlinked) instead of TestAddress3 (linked) for this test
	// TestAddress should be unlinked in the anvil state, so deploy will auto-link it
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey), "failed to create env file")
	require.NoError(t, createCliSettingsFile(tc, constants.TestAddress, "workflow-name", testEthUrl), "failed to create cre config file")
	require.NoError(t, createBlankProjectSettingFile(tc.ProjectDirectory+"project.yaml"), "failed to create project.yaml")
	t.Cleanup(tc.Cleanup(t))

	// Pre-baked registries from Anvil state dump
	t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
	t.Setenv(environments.EnvVarWorkflowRegistryChainName, TestChainName)
	t.Setenv(environments.EnvVarCapabilitiesRegistryAddress, "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9")
	t.Setenv(environments.EnvVarCapabilitiesRegistryChainName, TestChainName)

	// Use existing config file from test project
	_, thisFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(thisFile)
	configPath := filepath.Join(testDir, "test_project", "blank_workflow", "config.json")

	// Deploy workflow with unlinked address and config (should attempt auto-link but fail)
	deployOut := tc.workflowDeployUnlinkedWithConfig(t, configPath)
	require.Contains(t, deployOut, "Workflow compiled", "expected workflow to compile.\nCLI OUTPUT:\n%s", deployOut)

	// Key verification: The address should start unlinked, then attempt auto-linking
	// Check for the exact log messages from deploy.go ensureOwnerLinkedOrFail()
	require.Contains(t, deployOut, "Workflow owner link status", "expected owner link status message.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "linked=false", "expected initial link-status false.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "Owner not linked. Attempting auto-link...", "expected auto-link attempt message.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "Provide a label for your owner address", "expected label prompt.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "Starting linking", "expected linking start message.\nCLI OUTPUT:\n%s", deployOut)

	// The linking attempt should fail at the GraphQL request level
	// This simulates real-world failure due to signature validation issues
	require.Contains(t, deployOut, "auto-link attempt failed", "expected auto-link to fail.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "graphql request failed", "expected GraphQL request failure.\nCLI OUTPUT:\n%s", deployOut)

	// Deployment should fail since linking failed
	require.NotContains(t, deployOut, "Successfully uploaded workflow artifacts", "deployment should not succeed when linking fails.\nCLI OUTPUT:\n%s", deployOut)
	require.NotContains(t, deployOut, "Workflow deployed successfully", "deployment should not succeed when linking fails.\nCLI OUTPUT:\n%s", deployOut)
}

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

// Small struct to read GraphQL body
type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

// Deploys a workflow via CLI, mocking GraphQL + Origin.
func (tc *TestConfig) workflowDeployEoaWithMockStorage(t *testing.T) string {
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

	// Build CLI args
	args := []string{
		"workflow", "deploy",
		"main.go",
		tc.GetCliEnvFlag(),
		tc.GetCliSettingsFlag(),
		"--auto-start=true",
	}

	cmd := exec.Command(CLIPath, args...)
	cmd.Dir = wfDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow deploy failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	out := stripANSI(stdout.String() + stderr.String())

	return out
}

// Pauses all workflows (by owner + name) via CLI.
func (tc *TestConfig) workflowPauseEoa(t *testing.T) string {
	t.Helper()

	args := []string{
		"workflow", "pause",
		tc.GetCliEnvFlag(),
		tc.GetCliSettingsFlag(),
	}

	cmd := exec.Command(CLIPath, args...)
	// Run from the project root
	cmd.Dir = tc.ProjectDirectory

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow pause failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	return stripANSI(stdout.String() + stderr.String())
}

// Activates the workflow (by owner+name) via CLI.
func (tc *TestConfig) workflowActivateEoa(t *testing.T) string {
	t.Helper()

	args := []string{
		"workflow", "activate",
		tc.GetCliEnvFlag(),
		tc.GetCliSettingsFlag(),
	}

	cmd := exec.Command(CLIPath, args...)
	// Run from the project root
	cmd.Dir = tc.ProjectDirectory

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow activate failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	return stripANSI(stdout.String() + stderr.String())
}

// Deletes for the current owner+name via CLI (non-interactive).
func (tc *TestConfig) workflowDeleteEoa(t *testing.T) string {
	t.Helper()

	args := []string{
		"workflow", "delete",
		tc.GetCliEnvFlag(),
		tc.GetCliSettingsFlag(),
		"-y", // --skip-confirmation
	}

	cmd := exec.Command(CLIPath, args...)
	// Run from the project root
	cmd.Dir = tc.ProjectDirectory

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow delete failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	return stripANSI(stdout.String() + stderr.String())
}

func TestCLIWorkflowDeployWithEoa(t *testing.T) {
	// Start Anvil with pre-baked state
	anvilProc, testEthUrl := initTestEnv(t)
	defer StopAnvil(anvilProc)

	tc := NewTestConfig(t)

	// Use linked Address3 + its key
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3), "failed to create env file")
	require.NoError(t, createCliSettingsFile(tc, constants.TestAddress3, "workflow-name", testEthUrl), "failed to create cre config file")
	require.NoError(t, createBlankProjectSettingFile(tc.ProjectDirectory+"project.yaml"), "failed to create project.yaml")
	t.Cleanup(tc.Cleanup(t))

	// Pre-baked registries from Anvil state dump
	t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
	t.Setenv(environments.EnvVarWorkflowRegistryChainName, TestChainName)
	t.Setenv(environments.EnvVarCapabilitiesRegistryAddress, "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9")
	t.Setenv(environments.EnvVarCapabilitiesRegistryChainName, TestChainName)

	// Deploy with mocked storage
	out := tc.workflowDeployEoaWithMockStorage(t)
	require.Contains(t, out, "Workflow compiled", "expected workflow to compile.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow owner link status linked=true", "expected link-status true.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Successfully uploaded workflow artifacts", "expected upload to succeed.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow deployed successfully", "expected deployment success.\nCLI OUTPUT:\n%s", out)

	// Pause
	pauseOut := tc.workflowPauseEoa(t)
	require.Contains(t, pauseOut, "Workflows paused successfully", "pause should succeed.\nCLI OUTPUT:\n%s", pauseOut)

	// Activate
	activateOut := tc.workflowActivateEoa(t)
	require.Contains(t, activateOut, "Activating workflow", "should target latest workflow.\nCLI OUTPUT:\n%s", activateOut)
	require.Contains(t, activateOut, "Workflow activated successfully", "activate should succeed.\nCLI OUTPUT:\n%s", activateOut)

	// Delete
	deleteOut := tc.workflowDeleteEoa(t)
	require.Contains(t, deleteOut, "Workflows deleted successfully", "expected final deletion summary.\nCLI OUTPUT:\n%s", deleteOut)
}

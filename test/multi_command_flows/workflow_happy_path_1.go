package multi_command_flows

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// TestConfig represents test configuration
type TestConfig interface {
	GetCliEnvFlag() string
	GetProjectRootFlag() string
}

// CLI path for testing
var CLIPath = os.TempDir() + string(os.PathSeparator) + "cre" + func() string {
	if os.PathSeparator == '\\' {
		return ".exe"
	}
	return ""
}()

// Regular expression to strip ANSI escape codes from output
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI strips the ANSI escape codes from the output
func StripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// graphQLRequest represents a GraphQL request body
type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

// workflowDeployEoaWithMockStorage deploys a workflow via CLI, mocking GraphQL + Origin.
func workflowDeployEoaWithMockStorage(t *testing.T, tc TestConfig) string {
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

	// Build CLI args - CLI will automatically resolve workflow path using new context system
	args := []string{
		"workflow", "deploy",
		"blank_workflow",
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
		"--auto-start=true",
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	// Let CLI handle context switching - don't set cmd.Dir manually

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow deploy failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	out := StripANSI(stdout.String() + stderr.String())

	return out
}

// workflowPauseEoa pauses all workflows (by owner + name) via CLI.
func workflowPauseEoa(t *testing.T, tc TestConfig) string {
	t.Helper()

	args := []string{
		"workflow", "pause",
		"blank_workflow",
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	// CLI will handle context switching automatically

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow pause failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	return StripANSI(stdout.String() + stderr.String())
}

// workflowActivateEoa activates the workflow (by owner+name) via CLI.
func workflowActivateEoa(t *testing.T, tc TestConfig) string {
	t.Helper()

	args := []string{
		"workflow", "activate",
		"blank_workflow",
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	// CLI will handle context switching automatically

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow activate failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	return StripANSI(stdout.String() + stderr.String())
}

// workflowDeleteEoa deletes for the current owner+name via CLI (non-interactive).
func workflowDeleteEoa(t *testing.T, tc TestConfig) string {
	t.Helper()

	args := []string{
		"workflow", "delete",
		"blank_workflow",
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	// CLI will handle context switching automatically

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow delete failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	return StripANSI(stdout.String() + stderr.String())
}

// RunHappyPath1Workflow runs the complete happy path 1 workflow:
// Deploy -> Pause -> Activate -> Delete
func RunHappyPath1Workflow(t *testing.T, tc TestConfig) {
	t.Helper()

	// Set dummy API key
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	// Deploy with mocked storage
	out := workflowDeployEoaWithMockStorage(t, tc)
	require.Contains(t, out, "Workflow compiled", "expected workflow to compile.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow owner link status linked=true", "expected link-status true.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Successfully uploaded workflow artifacts", "expected upload to succeed.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow deployed successfully", "expected deployment success.\nCLI OUTPUT:\n%s", out)

	// Pause
	pauseOut := workflowPauseEoa(t, tc)
	require.Contains(t, pauseOut, "Workflows paused successfully", "pause should succeed.\nCLI OUTPUT:\n%s", pauseOut)

	// Activate
	activateOut := workflowActivateEoa(t, tc)
	require.Contains(t, activateOut, "Activating workflow", "should target latest workflow.\nCLI OUTPUT:\n%s", activateOut)
	require.Contains(t, activateOut, "Workflow activated successfully", "activate should succeed.\nCLI OUTPUT:\n%s", activateOut)

	// Delete
	deleteOut := workflowDeleteEoa(t, tc)
	require.Contains(t, deleteOut, "Workflows deleted successfully", "expected final deletion summary.\nCLI OUTPUT:\n%s", deleteOut)
}

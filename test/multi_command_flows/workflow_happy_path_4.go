package multi_command_flows

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// workflowDeployEoaWithMockStorage deploys a workflow via CLI, mocking GraphQL + Origin.
func setupMock(t *testing.T, tc TestConfig) (output string, gqlURL string) {
	t.Helper()

	var srv *httptest.Server
	// One server that handles both GraphQL and "origin" uploads.
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost:
			var req graphQLRequest
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
			if strings.Contains(req.Query, "listWorkflowOwners") {
				// Mock response for link verification check
				resp := map[string]any{
					"data": map[string]any{
						"listWorkflowOwners": map[string]any{
							"linkedOwners": []map[string]string{
								{
									"workflowOwnerAddress": "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC",
									"verificationStatus":   "VERIFICATION_STATUS_SUCCESSFULL", //nolint:misspell // Intentional misspelling to match external API
								},
							},
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
	// Note: Server is NOT closed here - caller is responsible for keeping it alive
	// across multiple commands. The server should be closed at the end of the test.

	// Point the CLI at our mock GraphQL endpoint
	gqlURL = srv.URL + "/graphql"
	t.Setenv(environments.EnvVarGraphQLURL, gqlURL)

	return
}

func deployWorkflow(t *testing.T, tc TestConfig, workflowName string) (output string, err error) {
	// Build CLI args - CLI will automatically resolve workflow path using new context system
	args := []string{
		"workflow", "deploy",
		workflowName,
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	// Let CLI handle context switching - don't set cmd.Dir manually

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	err = cmd.Run()

	output = StripANSI(stdout.String() + stderr.String())
	return
}

// DuplicateDeployRejected deploys a workflow and then attempts to deploy it again, confirming that it fails.
func DuplicateDeployRejected(t *testing.T, tc TestConfig, workflowName string) {
	t.Helper()

	// Set dummy API key
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	setupMock(t, tc)

	// Deploy with mocked storage - this creates the server and returns the GraphQL URL
	out, err := deployWorkflow(t, tc, workflowName)
	if err != nil {
		t.Fatalf("failed to deploy workflow: %v", err)
	}
	require.Contains(t, out, "Workflow compiled", "expected workflow to compile.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "linked=true", "expected link-status true.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Uploaded binary", "expected binary upload to succeed.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow deployed successfully", "expected deployment success.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Preparing transaction for workflowID:", "expected transaction preparation.\nCLI OUTPUT:\n%s", out)
	// extract the workflowID from the output (only the ID, not following lines)
	afterPrefix := strings.Split(out, "Preparing transaction for workflowID:")[1]
	workflowID := strings.TrimSpace(strings.Split(afterPrefix, "\n")[0])
	t.Logf("workflowID: %s", workflowID)
	require.NotEmpty(t, workflowID, "expected workflowID to be not empty.\nCLI OUTPUT:\n%s", out)

	// deploy workflow again and confirm it fails
	out2, _ := deployWorkflow(t, tc, workflowName) // ignore error, we expect it to fail
	require.Contains(t, out2, "workflow with id "+workflowID+" already exists", "expected workflow to be already deployed.\nCLI OUTPUT:\n%s", out2)
}

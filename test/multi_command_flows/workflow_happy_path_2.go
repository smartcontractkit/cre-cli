package multi_command_flows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// workflowDeployEoaWithoutAutostart deploys a workflow via CLI without autostart, mocking GraphQL + Origin.
func workflowDeployEoaWithoutAutostart(t *testing.T, tc TestConfig) string {
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
	// Note: no auto-start flag (defaults to false)
	args := []string{
		"workflow", "deploy",
		"blank_workflow",
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
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

// workflowDeployUpdateWithConfig deploys a workflow update with config via CLI, mocking GraphQL + Origin.
func workflowDeployUpdateWithConfig(t *testing.T, tc TestConfig) string {
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

	// Build CLI args with config file - CLI will automatically resolve workflow path
	args := []string{
		"workflow", "deploy",
		"blank_workflow",
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	// Let CLI handle context switching - don't set cmd.Dir manually

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow deploy update failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	out := StripANSI(stdout.String() + stderr.String())

	return out
}

// RunHappyPath2Workflow runs the complete happy path 2 workflow:
// Deploy without autostart -> Deploy update with config
func RunHappyPath2Workflow(t *testing.T, tc TestConfig) {
	t.Helper()

	// Step 1: Deploy initial workflow without autostart
	out := workflowDeployEoaWithoutAutostart(t, tc)
	require.Contains(t, out, "Workflow compiled", "expected workflow to compile.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "linked=true", "expected link-status true.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Uploaded binary", "expected binary upload to succeed.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow deployed successfully", "expected deployment success.\nCLI OUTPUT:\n%s", out)

	// Step 1.5: Update workflow.yaml to include config-path for the second deployment
	// This ensures the second deployment has different artifacts and generates a different workflowID
	if err := updateWorkflowConfigPath(tc.GetProjectRootFlag(), "./config.json"); err != nil {
		require.NoError(t, err, "failed to update workflow config path")
	}

	// Step 2: Deploy update with config (workflow already setup from step 1)
	updateOut := workflowDeployUpdateWithConfig(t, tc)
	require.Contains(t, updateOut, "Workflow compiled", "expected workflow to compile on update.\nCLI OUTPUT:\n%s", updateOut)
	require.Contains(t, updateOut, "linked=true", "expected link-status true.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Uploaded binary", "expected config upload to succeed.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, updateOut, "Workflow deployed successfully", "expected deployment update success.\nCLI OUTPUT:\n%s", updateOut)
}

// updateWorkflowConfigPath updates the config-path in the workflow.yaml file
func updateWorkflowConfigPath(projectRootFlag, configPath string) error {
	const SettingsTarget = "staging"

	// Extract directory path from flag format "--project-root=/path/..."
	parts := strings.Split(projectRootFlag, "=")
	if len(parts) != 2 {
		return fmt.Errorf("invalid project root flag format: %s", projectRootFlag)
	}
	projectDirectory := parts[1]

	workflowDir := filepath.Join(projectDirectory, "blank_workflow")
	workflowSettingsPath := filepath.Join(workflowDir, constants.DefaultWorkflowSettingsFileName)

	v := viper.New()
	v.SetConfigFile(workflowSettingsPath)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read workflow.yaml: %w", err)
	}

	// Update the config-path in workflow-artifacts
	workflowArtifacts := v.GetStringMapString(fmt.Sprintf("%s.workflow-artifacts", SettingsTarget))
	if workflowArtifacts == nil {
		workflowArtifacts = make(map[string]string)
	}

	workflowArtifacts["workflow-path"] = "./main.go"
	workflowArtifacts["config-path"] = configPath
	v.Set(fmt.Sprintf("%s.workflow-artifacts", SettingsTarget), workflowArtifacts)

	// Write the updated configuration
	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write workflow.yaml: %w", err)
	}

	return nil
}

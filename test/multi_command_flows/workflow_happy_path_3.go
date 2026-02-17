package multi_command_flows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// workflowInit runs cre init to initialize a new workflow project from scratch
func workflowInit(t *testing.T, projectRootFlag, projectName, workflowName string) (output string, gqlURL string) {
	t.Helper()

	// Set up mock GraphQL server for authentication validation
	gqlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost {
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

			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
			})
		}
	}))
	// Note: Server is NOT closed here - caller is responsible for keeping it alive
	// across multiple commands. The server should be closed at the end of the test.

	// Point GraphQL client to mock server
	gqlURL = gqlSrv.URL + "/graphql"
	t.Setenv(environments.EnvVarGraphQLURL, gqlURL)

	args := []string{
		"init",
		"--project-name", projectName,
		"--workflow-name", workflowName,
		"--template-id", "2", // Use blank template (ID 2)
	}

	cmd := exec.Command(CLIPath, args...)

	// Set working directory to where the project should be created
	parts := strings.Split(projectRootFlag, "=")
	require.Len(t, parts, 2, "invalid project root flag format")
	cmd.Dir = parts[1]

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre init failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)

	output = StripANSI(stdout.String() + stderr.String())
	return
}

// workflowDeployUnsigned deploys with --unsigned flag to test auto-link initiation without contract submission
func workflowDeployUnsigned(t *testing.T, tc TestConfig, projectRootFlag, workflowName string) (string, error) {
	t.Helper()

	var srv *httptest.Server
	// One server that handles both GraphQL and "origin" uploads, plus auto-link
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

			// Handle initiateLinking mutation for auto-link
			if strings.Contains(req.Query, "initiateLinking") {
				resp := map[string]any{
					"data": map[string]any{
						"initiateLinking": map[string]any{
							"ownershipProofHash":   "0x1234567890123456789012345678901234567890123456789012345678901234",
							"workflowOwnerAddress": req.Variables["request"].(map[string]interface{})["workflowOwnerAddress"],
							"validUntil":           "2099-12-31T23:59:59Z",
							"signature":            "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567801",
							"chainSelector":        "6433500567565415381",
							"contractAddress":      "0x5FbDB2315678afecb367f032d93F642f64180aa3",
							"transactionData":      "0x",
							"functionSignature":    "linkOwner",
							"functionArgs":         []string{},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
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
	t.Setenv(environments.EnvVarGraphQLURL, srv.URL+"/graphql")

	// Build CLI args with --unsigned flag to avoid contract submission
	args := []string{
		"workflow", "deploy",
		workflowName,
		tc.GetCliEnvFlag(),
		projectRootFlag,
		"--unsigned",
		"--owner-label", "test-owner-label",
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	// Let CLI handle context switching - don't set cmd.Dir manually

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	err := cmd.Run()
	out := StripANSI(stdout.String() + stderr.String())

	return out, err
}

// workflowDeployWithConfigAndLinkedKey deploys a workflow with config using a pre-linked address
func workflowDeployWithConfigAndLinkedKey(t *testing.T, tc TestConfig, projectRootFlag, workflowName string) string {
	t.Helper()

	var srv *httptest.Server
	// One server that handles both GraphQL and "origin" uploads
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

			// Handle listWorkflowOwners query for link verification
			if strings.Contains(req.Query, "listWorkflowOwners") {
				// Return the owner as linked and verified
				resp := map[string]any{
					"data": map[string]any{
						"listWorkflowOwners": map[string]any{
							"linkedOwners": []map[string]string{
								{
									"workflowOwnerAddress": "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC",
									"verificationStatus":   "VERIFICATION_STATUS_SUCCESSFULL", //nolint
								},
							},
						},
					},
				}
				_ = json.NewEncoder(w).Encode(resp)
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
	t.Setenv(environments.EnvVarGraphQLURL, srv.URL+"/graphql")

	// Build CLI args - CLI will automatically resolve workflow path using new context system
	args := []string{
		"workflow", "deploy",
		workflowName,
		tc.GetCliEnvFlag(),
		projectRootFlag,
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

// updateProjectSettings updates the project.yaml file with test settings
func updateProjectSettings(projectRootFlag, ownerAddress, ethUrl string) error {
	const SettingsTarget = "staging-settings"

	// Extract directory path from flag format "--project-root=/path/..."
	parts := strings.Split(projectRootFlag, "=")
	if len(parts) != 2 {
		return fmt.Errorf("invalid project root flag format: %s", projectRootFlag)
	}
	projectDirectory := parts[1]

	projectSettingsPath := filepath.Join(projectDirectory, "project.yaml")

	v := viper.New()
	v.SetConfigFile(projectSettingsPath)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read project.yaml: %w", err)
	}

	// Update account settings
	v.Set(fmt.Sprintf("%s.account.workflow-owner-address", SettingsTarget), ownerAddress)

	// Update RPC settings
	v.Set(fmt.Sprintf("%s.rpcs", SettingsTarget), []map[string]string{
		{
			"chain-name": "anvil-devnet",
			"url":        ethUrl,
		},
	})

	// Write the updated configuration
	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write project.yaml: %w", err)
	}

	return nil
}

// RunHappyPath3aWorkflow runs happy path 3a: Init → Deploy with unlinked key (tests auto-link initiation)
// This test validates the auto-link flow is triggered but uses --unsigned to avoid contract verification
func RunHappyPath3aWorkflow(t *testing.T, tc TestConfig, projectName, ownerAddress, ethUrl string) {
	t.Helper()

	workflowName := "happy-path-3a-workflow"

	// Step 1: Initialize new project with workflow
	initOut, gqlURL := workflowInit(t, tc.GetProjectRootFlag(), projectName, workflowName)
	require.Contains(t, initOut, "Project created successfully", "expected init to succeed.\nCLI OUTPUT:\n%s", initOut)

	// Build the project root flag pointing to the newly created project
	parts := strings.Split(tc.GetProjectRootFlag(), "=")
	require.Len(t, parts, 2, "invalid project root flag format")
	baseDir := parts[1]
	projectRootFlag := fmt.Sprintf("--project-root=%s%s/", baseDir, projectName)

	// Step 2: Update project.yaml with correct test settings
	if err := updateProjectSettings(projectRootFlag, ownerAddress, ethUrl); err != nil {
		require.NoError(t, err, "failed to update project settings")
	}

	// Step 3: Deploy with unlinked key using --unsigned flag to avoid contract submission
	// Reuse the same GraphQL server from init
	t.Setenv(environments.EnvVarGraphQLURL, gqlURL)
	deployOut, deployErr := workflowDeployUnsigned(t, tc, projectRootFlag, workflowName)

	// Verify auto-link flow was triggered
	require.Contains(t, deployOut, "Workflow compiled", "expected workflow to compile.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "linked=false", "expected link-status false initially.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "attempting auto-link", "expected auto-link attempt.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "Linking web3 key", "expected auto-link to start.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "Starting linking", "expected linking process to begin.\nCLI OUTPUT:\n%s", deployOut)

	// With --unsigned flag and mock signature, auto-link will fail at contract verification
	// This is expected - we've validated the auto-link flow initiation
	require.Error(t, deployErr, "expected deploy to fail due to mock signature verification")
	require.Contains(t, deployOut, "auto-link attempt failed", "expected auto-link failure message.\nCLI OUTPUT:\n%s", deployOut)
}

// RunHappyPath3bWorkflow runs happy path 3b: Deploy with linked key + config
// This test validates successful deployment with config when the key is already linked
// Note: The workflow directory is created with config-path already set by createWorkflowDirectory
func RunHappyPath3bWorkflow(t *testing.T, tc TestConfig) {
	t.Helper()

	// The workflow directory name matches the template directory (blank_workflow)
	workflowDirectoryName := "blank_workflow"

	// Deploy with config using pre-linked address (config-path already set during directory creation)
	deployOut := workflowDeployWithConfigAndLinkedKey(t, tc, tc.GetProjectRootFlag(), workflowDirectoryName)
	require.Contains(t, deployOut, "Workflow compiled", "expected workflow to compile.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "linked=true", "expected link-status true.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "Uploaded binary", "expected binary upload to succeed.\nCLI OUTPUT:\n%s", deployOut)
	require.Contains(t, deployOut, "Workflow deployed successfully", "expected deployment success.\nCLI OUTPUT:\n%s", deployOut)
}

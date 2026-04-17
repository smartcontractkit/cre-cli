package multi_command_flows

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

const privateRegistryOwnerAddress = "0x0bb6e890f43f93f4f4f5eb64fdf17f81f15bf12a"

func createTestBearerCredentialsHome(t *testing.T) string {
	t.Helper()

	homeDir := t.TempDir()
	creDir := filepath.Join(homeDir, ".cre")
	require.NoError(t, os.MkdirAll(creDir, 0o700), "failed to create .cre dir")

	jwt := createTestJWT("test-org-id")
	creConfig := "AccessToken: " + jwt + "\n" +
		"IDToken: test-id-token\n" +
		"RefreshToken: test-refresh-token\n" +
		"ExpiresIn: 3600\n" +
		"TokenType: Bearer\n"

	require.NoError(t, os.WriteFile(filepath.Join(creDir, "cre.yaml"), []byte(creConfig), 0o600), "failed to write test credentials")

	return homeDir
}

func createTestJWT(orgID string) string {
	header, _ := json.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{
		"sub":                 "test-user",
		"org_id":              orgID,
		"organization_status": "FULL_ACCESS",
		"exp":                 time.Now().Add(2 * time.Hour).Unix(),
	})

	headerEnc := base64.RawURLEncoding.EncodeToString(header)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payload)

	return headerEnc + "." + payloadEnc + ".signature"
}

// workflowDeployPrivateRegistry deploys a workflow to the private registry via CLI
// using a mock GraphQL + upload server.
func workflowDeployPrivateRegistry(t *testing.T, tc TestConfig) string {
	t.Helper()

	var presignedPostCalled atomic.Bool
	var uploadCalled atomic.Bool
	var upsertCalled atomic.Bool
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost:
			var req graphQLRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")

			if strings.Contains(req.Query, "getCreOrganizationInfo") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getCreOrganizationInfo": map[string]any{
							"orgId":                 "test-org-id",
							"derivedWorkflowOwners": []string{"ab12cd34ef56ab12cd34ef56ab12cd34ef56ab12"},
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "GetTenantConfig") || strings.Contains(req.Query, "getTenantConfig") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getTenantConfig": map[string]any{
							"tenantId":         "42",
							"defaultDonFamily": "test-don",
							"vaultGatewayUrl":  "https://vault.example.test",
							"registries": []map[string]any{
								{
									"id":               "reg-test",
									"label":            "reg-test",
									"type":             "ON_CHAIN",
									"chainSelector":    "6433500567565415381",
									"address":          "0x5FbDB2315678afecb367f032d93F642f64180aa3",
									"secretsAuthFlows": []string{"BROWSER"},
								},
							},
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "GeneratePresignedPostUrlForArtifact") {
				presignedPostCalled.Store(true)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"generatePresignedPostUrlForArtifact": map[string]any{
							"presignedPostUrl":    srv.URL + "/upload",
							"presignedPostFields": []map[string]string{{"key": "k1", "value": "v1"}},
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "GenerateUnsignedGetUrlForArtifact") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"generateUnsignedGetUrlForArtifact": map[string]any{
							"unsignedGetUrl": srv.URL + "/get/binary.wasm",
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "listWorkflowOwners") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"listWorkflowOwners": map[string]any{
							"linkedOwners": []map[string]string{
								{
									"workflowOwnerAddress": "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC",
									"verificationStatus":   "VERIFICATION_STATUS_SUCCESSFULL", //nolint:misspell
								},
							},
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "GetOffchainWorkflowByName") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]any{
						{
							"message": "workflow not found",
							"path":    []string{"getOffchainWorkflowByName"},
							"extensions": map[string]any{
								"code": "NOT_FOUND",
							},
						},
					},
					"data": nil,
				})
				return
			}

			if strings.Contains(req.Query, "UpsertOffchainWorkflow") {
				upsertCalled.Store(true)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"upsertOffchainWorkflow": map[string]any{
							"workflow": map[string]any{
								"workflowId":     "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
								"owner":          privateRegistryOwnerAddress,
								"createdAt":      "2025-01-01T00:00:00Z",
								"status":         "WORKFLOW_STATUS_ACTIVE",
								"workflowName":   "private-registry-happy-path-workflow",
								"binaryUrl":      srv.URL + "/get/binary.wasm",
								"configUrl":      "",
								"tag":            "private-registry-happy-path-workflow",
								"attributes":     "",
								"donFamily":      "test-don",
								"organizationId": "test-org-id",
							},
						},
					},
				})
				return
			}

			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
			})
			return

		case r.URL.Path == "/upload" && r.Method == http.MethodPost:
			uploadCalled.Store(true)
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

	t.Setenv(environments.EnvVarGraphQLURL, srv.URL+"/graphql")

	args := []string{
		"workflow", "deploy",
		"blank_workflow",
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
		"--preview-private-registry",
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	testHome := createTestBearerCredentialsHome(t)

	realHome, err := os.UserHomeDir()
	require.NoError(t, err, "failed to get real home dir")

	childEnv := make([]string, 0, len(os.Environ())+3)
	hasGOPATH := false
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "HOME=") || strings.HasPrefix(entry, "USERPROFILE=") {
			continue
		}
		if strings.HasPrefix(entry, "GOPATH=") {
			hasGOPATH = true
		}
		childEnv = append(childEnv, entry)
	}
	childEnv = append(childEnv, "HOME="+testHome, "USERPROFILE="+testHome)
	// When HOME is overridden, Go defaults GOPATH to $HOME/go which lands
	// inside t.TempDir(). Go modules are read-only, so TempDir cleanup
	// fails and marks the test as failed. Pin GOPATH to the real home.
	if !hasGOPATH {
		childEnv = append(childEnv, "GOPATH="+filepath.Join(realHome, "go"))
	}
	cmd.Env = childEnv

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow deploy --preview-private-registry failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)
	require.True(t, presignedPostCalled.Load(), "expected GeneratePresignedPostUrlForArtifact to be called")
	require.True(t, uploadCalled.Load(), "expected artifact upload endpoint to be called")
	require.True(t, upsertCalled.Load(), "expected UpsertOffchainWorkflow to be called")

	return StripANSI(stdout.String() + stderr.String())
}

// RunWorkflowPrivateRegistryHappyPath runs the workflow deploy happy path for private registry.
func RunWorkflowPrivateRegistryHappyPath(t *testing.T, tc TestConfig) {
	t.Helper()

	out := workflowDeployPrivateRegistry(t, tc)
	require.Contains(t, out, "Workflow compiled", "expected workflow to compile.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Uploaded binary", "expected binary upload to succeed.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow registered in private registry", "expected private registry deployment success.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Details:", "expected details block.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow Name: private-registry-happy-path-workflow", "expected workflow name in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow ID:", "expected workflow ID in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Status:        WORKFLOW_STATUS_ACTIVE", "expected active status in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Binary URL:", "expected binary URL in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Owner:         "+privateRegistryOwnerAddress, "expected owner in details.\nCLI OUTPUT:\n%s", out)
}

// workflowPausePrivateRegistry pauses a workflow in the private registry via CLI
// using a mock GraphQL server.
func workflowPausePrivateRegistry(t *testing.T, tc TestConfig) string {
	t.Helper()

	var getWorkflowCalled atomic.Bool
	var pauseWorkflowCalled atomic.Bool
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost:
			var req graphQLRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")

			if strings.Contains(req.Query, "getCreOrganizationInfo") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getCreOrganizationInfo": map[string]any{
							"orgId":                 "test-org-id",
							"derivedWorkflowOwners": []string{"ab12cd34ef56ab12cd34ef56ab12cd34ef56ab12"},
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "GetTenantConfig") || strings.Contains(req.Query, "getTenantConfig") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getTenantConfig": map[string]any{
							"tenantId":         "42",
							"defaultDonFamily": "test-don",
							"vaultGatewayUrl":  "https://vault.example.test",
							"registries": []map[string]any{
								{
									"id":               "reg-test",
									"label":            "reg-test",
									"type":             "OFF_CHAIN",
									"chainSelector":    "6433500567565415381",
									"address":          "0x5FbDB2315678afecb367f032d93F642f64180aa3",
									"secretsAuthFlows": []string{"BROWSER"},
								},
							},
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "GetOffchainWorkflowByName") {
				getWorkflowCalled.Store(true)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getOffchainWorkflowByName": map[string]any{
							"workflow": map[string]any{
								"workflowId":     "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
								"owner":          privateRegistryOwnerAddress,
								"createdAt":      "2025-01-01T00:00:00Z",
								"status":         "WORKFLOW_STATUS_ACTIVE",
								"workflowName":   "private-registry-happy-path-workflow",
								"binaryUrl":      srv.URL + "/get/binary.wasm",
								"configUrl":      "",
								"tag":            "private-registry-happy-path-workflow",
								"attributes":     "",
								"donFamily":      "test-don",
								"organizationId": "test-org-id",
							},
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "PauseOffchainWorkflow") {
				pauseWorkflowCalled.Store(true)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"pauseOffchainWorkflow": map[string]any{
							"workflow": map[string]any{
								"workflowId":     "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
								"owner":          privateRegistryOwnerAddress,
								"createdAt":      "2025-01-01T00:00:00Z",
								"status":         "WORKFLOW_STATUS_PAUSED",
								"workflowName":   "private-registry-happy-path-workflow",
								"binaryUrl":      srv.URL + "/get/binary.wasm",
								"configUrl":      "",
								"tag":            "private-registry-happy-path-workflow",
								"attributes":     "",
								"donFamily":      "test-don",
								"organizationId": "test-org-id",
							},
						},
					},
				})
				return
			}

			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
			})
			return

		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
			return
		}
	}))
	defer srv.Close()

	t.Setenv(environments.EnvVarGraphQLURL, srv.URL+"/graphql")

	args := []string{
		"workflow", "pause",
		"blank_workflow",
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	testHome := createTestBearerCredentialsHome(t)

	realHome, err := os.UserHomeDir()
	require.NoError(t, err, "failed to get real home dir")

	childEnv := make([]string, 0, len(os.Environ())+3)
	hasGOPATH := false
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "HOME=") || strings.HasPrefix(entry, "USERPROFILE=") {
			continue
		}
		if strings.HasPrefix(entry, "GOPATH=") {
			hasGOPATH = true
		}
		childEnv = append(childEnv, entry)
	}
	childEnv = append(childEnv, "HOME="+testHome, "USERPROFILE="+testHome)
	if !hasGOPATH {
		childEnv = append(childEnv, "GOPATH="+filepath.Join(realHome, "go"))
	}
	cmd.Env = childEnv

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow pause failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)
	require.True(t, getWorkflowCalled.Load(), "expected GetOffchainWorkflowByName to be called")
	require.True(t, pauseWorkflowCalled.Load(), "expected PauseOffchainWorkflow to be called")

	return StripANSI(stdout.String() + stderr.String())
}

// RunWorkflowPausePrivateRegistryHappyPath runs the workflow pause happy path for private registry.
func RunWorkflowPausePrivateRegistryHappyPath(t *testing.T, tc TestConfig) {
	t.Helper()

	projectRoot := strings.TrimPrefix(tc.GetProjectRootFlag(), "--project-root=")
	workflowYamlPath := filepath.Join(projectRoot, "blank_workflow", "workflow.yaml")
	
	v := viper.New()
	v.SetConfigFile(workflowYamlPath)
	err := v.ReadInConfig()
	require.NoError(t, err)
	
	v.Set("staging-settings.user-workflow.deployment-registry", "reg-test")
	err = v.WriteConfig()
	require.NoError(t, err)

	out := workflowPausePrivateRegistry(t, tc)
	require.Contains(t, out, "Workflow paused successfully", "expected private registry pause success.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Details:", "expected details block.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow Name: private-registry-happy-path-workflow", "expected workflow name in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow ID:", "expected workflow ID in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Status:        WORKFLOW_STATUS_PAUSED", "expected paused status in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Owner:         "+privateRegistryOwnerAddress, "expected owner in details.\nCLI OUTPUT:\n%s", out)
}

// workflowActivatePrivateRegistry activates a workflow in the private registry via CLI
// using a mock GraphQL server.
func workflowActivatePrivateRegistry(t *testing.T, tc TestConfig) string {
	t.Helper()

	var getWorkflowCalled atomic.Bool
	var activateWorkflowCalled atomic.Bool
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost:
			var req graphQLRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")

			if strings.Contains(req.Query, "getCreOrganizationInfo") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getCreOrganizationInfo": map[string]any{
							"orgId":                 "test-org-id",
							"derivedWorkflowOwners": []string{"ab12cd34ef56ab12cd34ef56ab12cd34ef56ab12"},
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "GetTenantConfig") || strings.Contains(req.Query, "getTenantConfig") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getTenantConfig": map[string]any{
							"tenantId":         "42",
							"defaultDonFamily": "test-don",
							"vaultGatewayUrl":  "https://vault.example.test",
							"registries": []map[string]any{
								{
									"id":               "reg-test",
									"label":            "reg-test",
									"type":             "OFF_CHAIN",
									"chainSelector":    "6433500567565415381",
									"address":          "0x5FbDB2315678afecb367f032d93F642f64180aa3",
									"secretsAuthFlows": []string{"BROWSER"},
								},
							},
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "GetOffchainWorkflowByName") {
				getWorkflowCalled.Store(true)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getOffchainWorkflowByName": map[string]any{
							"workflow": map[string]any{
								"workflowId":     "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
								"owner":          privateRegistryOwnerAddress,
								"createdAt":      "2025-01-01T00:00:00Z",
								"status":         "WORKFLOW_STATUS_PAUSED",
								"workflowName":   "private-registry-happy-path-workflow",
								"binaryUrl":      srv.URL + "/get/binary.wasm",
								"configUrl":      "",
								"tag":            "private-registry-happy-path-workflow",
								"attributes":     "",
								"donFamily":      "test-don",
								"organizationId": "test-org-id",
							},
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "ActivateOffchainWorkflow") {
				activateWorkflowCalled.Store(true)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"activateOffchainWorkflow": map[string]any{
							"workflow": map[string]any{
								"workflowId":     "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
								"owner":          privateRegistryOwnerAddress,
								"createdAt":      "2025-01-01T00:00:00Z",
								"status":         "WORKFLOW_STATUS_ACTIVE",
								"workflowName":   "private-registry-happy-path-workflow",
								"binaryUrl":      srv.URL + "/get/binary.wasm",
								"configUrl":      "",
								"tag":            "private-registry-happy-path-workflow",
								"attributes":     "",
								"donFamily":      "test-don",
								"organizationId": "test-org-id",
							},
						},
					},
				})
				return
			}

			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
			})
			return

		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
			return
		}
	}))
	defer srv.Close()

	t.Setenv(environments.EnvVarGraphQLURL, srv.URL+"/graphql")

	args := []string{
		"workflow", "activate",
		"blank_workflow",
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	testHome := createTestBearerCredentialsHome(t)

	realHome, err := os.UserHomeDir()
	require.NoError(t, err, "failed to get real home dir")

	childEnv := make([]string, 0, len(os.Environ())+3)
	hasGOPATH := false
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "HOME=") || strings.HasPrefix(entry, "USERPROFILE=") {
			continue
		}
		if strings.HasPrefix(entry, "GOPATH=") {
			hasGOPATH = true
		}
		childEnv = append(childEnv, entry)
	}
	childEnv = append(childEnv, "HOME="+testHome, "USERPROFILE="+testHome)
	if !hasGOPATH {
		childEnv = append(childEnv, "GOPATH="+filepath.Join(realHome, "go"))
	}
	cmd.Env = childEnv

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre workflow activate failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)
	require.True(t, getWorkflowCalled.Load(), "expected GetOffchainWorkflowByName to be called")
	require.True(t, activateWorkflowCalled.Load(), "expected ActivateOffchainWorkflow to be called")

	return StripANSI(stdout.String() + stderr.String())
}

// RunWorkflowActivatePrivateRegistryHappyPath runs the workflow activate happy path for private registry.
func RunWorkflowActivatePrivateRegistryHappyPath(t *testing.T, tc TestConfig) {
	t.Helper()

	out := workflowActivatePrivateRegistry(t, tc)
	require.Contains(t, out, "Workflow activated successfully", "expected private registry activate success.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Details:", "expected details block.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow Name: private-registry-happy-path-workflow", "expected workflow name in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow ID:", "expected workflow ID in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Status:        WORKFLOW_STATUS_ACTIVE", "expected active status in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Owner:         "+privateRegistryOwnerAddress, "expected owner in details.\nCLI OUTPUT:\n%s", out)
}

package multi_command_flows

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/authvalidation"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/ethkeys"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
	"github.com/smartcontractkit/cre-cli/internal/testutil/testjwt"
)

// MockDerivedWorkflowOwnerHex is the raw address (no 0x) in getCreOrganizationInfo.derivedWorkflowOwners
// in tests. The canonical checksummed form is privateRegistryOwnerAddress.
const MockDerivedWorkflowOwnerHex = "ab12cd34ef56ab12cd34ef56ab12cd34ef56ab12"

var privateRegistryOwnerAddress = mustChecksumMockDerivedOwner()

func mustChecksumMockDerivedOwner() string {
	s, err := ethkeys.FormatWorkflowOwnerAddress("0x" + MockDerivedWorkflowOwnerHex)
	if err != nil {
		panic(err)
	}
	return s
}

// mockGetCreOrganizationInfoGraphQLPayload is the full GraphQL JSON body for getCreOrganizationInfo
// (shared by composite mocks and graphQLMockOrgInfoOnly).
func mockGetCreOrganizationInfoGraphQLPayload() map[string]any {
	return map[string]any{
		"data": map[string]any{
			"getCreOrganizationInfo": map[string]any{
				"orgId":                 "test-org-id",
				"derivedWorkflowOwners": []string{MockDerivedWorkflowOwnerHex},
			},
		},
	}
}

// CreateTestBearerCredentialsHome writes JWT bearer credentials under HOME/.cre for subprocess CLI tests.
func CreateTestBearerCredentialsHome(t *testing.T) string {
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
	return testjwt.CreateTestJWT(orgID)
}

// workflowDeployPrivateRegistry deploys a workflow to the private registry via CLI.
// It starts a single httptest.Server that mocks GraphQL (getCreOrganizationInfo,
// GetTenantConfig, GeneratePresignedPostUrlForArtifact, GenerateUnsignedGetUrlForArtifact,
// listWorkflowOwners, GetOffchainWorkflowByName, UpsertOffchainWorkflow) and POST /upload.
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
				_ = json.NewEncoder(w).Encode(mockGetCreOrganizationInfoGraphQLPayload())
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
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	testHome := CreateTestBearerCredentialsHome(t)

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
		"cre workflow deploy failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
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
	require.Contains(t, out, "Registry:         reg-test", "expected registry ID in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow Name:    private-registry-happy-path-workflow", "expected workflow name in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow ID:", "expected workflow ID in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Status:           Active", "expected active status in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Binary URL:", "expected binary URL in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Owner:            "+privateRegistryOwnerAddress, "expected owner in details.\nCLI OUTPUT:\n%s", out)
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
				_ = json.NewEncoder(w).Encode(mockGetCreOrganizationInfoGraphQLPayload())
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
	testHome := CreateTestBearerCredentialsHome(t)

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

	out := workflowPausePrivateRegistry(t, tc)
	require.Contains(t, out, "Workflow paused successfully", "expected private registry pause success.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Details:", "expected details block.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Registry:         reg-test", "expected registry ID in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow Name:    private-registry-happy-path-workflow", "expected workflow name in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow ID:", "expected workflow ID in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Status:           Paused", "expected paused status in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Owner:            "+privateRegistryOwnerAddress, "expected owner in details.\nCLI OUTPUT:\n%s", out)
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
				_ = json.NewEncoder(w).Encode(mockGetCreOrganizationInfoGraphQLPayload())
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
	testHome := CreateTestBearerCredentialsHome(t)

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
	require.Contains(t, out, "Registry:         reg-test", "expected registry ID in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow Name:    private-registry-happy-path-workflow", "expected workflow name in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow ID:", "expected workflow ID in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Status:           Active", "expected active status in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Owner:            "+privateRegistryOwnerAddress, "expected owner in details.\nCLI OUTPUT:\n%s", out)
}

// workflowDeletePrivateRegistry deletes a workflow in the private registry via CLI
// using a mock GraphQL server.
func workflowDeletePrivateRegistry(t *testing.T, tc TestConfig) string {
	t.Helper()

	var getWorkflowCalled atomic.Bool
	var deleteWorkflowCalled atomic.Bool
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost:
			var req graphQLRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")

			if strings.Contains(req.Query, "getCreOrganizationInfo") {
				_ = json.NewEncoder(w).Encode(mockGetCreOrganizationInfoGraphQLPayload())
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

			if strings.Contains(req.Query, "DeleteOffchainWorkflow") {
				deleteWorkflowCalled.Store(true)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"deleteOffchainWorkflow": map[string]any{
							"workflowId": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
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
		"workflow", "delete",
		"blank_workflow",
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
		"--" + settings.Flags.SkipConfirmation.Name,
	}

	cmd := exec.Command(CLIPath, args...)
	testHome := CreateTestBearerCredentialsHome(t)

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
		"cre workflow delete failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		stdout.String(),
		stderr.String(),
	)
	require.True(t, getWorkflowCalled.Load(), "expected GetOffchainWorkflowByName to be called")
	require.True(t, deleteWorkflowCalled.Load(), "expected DeleteOffchainWorkflow to be called")

	return StripANSI(stdout.String() + stderr.String())
}

// RunWorkflowDeletePrivateRegistryHappyPath runs the workflow delete happy path for private registry.
func RunWorkflowDeletePrivateRegistryHappyPath(t *testing.T, tc TestConfig) {
	t.Helper()

	out := workflowDeletePrivateRegistry(t, tc)
	require.Contains(t, out, "Workflows deleted successfully", "expected private registry delete success.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Registry:         reg-test", "expected registry ID in delete preview.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow Status:  Active", "expected formatted status in delete preview.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Deleted workflow ID: 1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", "expected workflow ID in details.\nCLI OUTPUT:\n%s", out)
}

func workflowDeployCmd() *cobra.Command {
	w := &cobra.Command{Use: "workflow"}
	d := &cobra.Command{Use: "deploy"}
	w.AddCommand(d)
	return d
}

func assertEnvFileHasNoEthPrivateKey(t *testing.T, envPath string) {
	t.Helper()
	b, err := os.ReadFile(envPath)
	require.NoError(t, err)
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), settings.EthPrivateKeyEnvVar) {
			if strings.TrimSpace(val) != "" {
				t.Fatalf("expected %s to be unset in %s, got a non-empty value", settings.EthPrivateKeyEnvVar, envPath)
			}
		}
	}
}

func graphQLMockOrgInfoOnly(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/graphql") || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.Query, "getCreOrganizationInfo") {
			require.NoError(t, json.NewEncoder(w).Encode(mockGetCreOrganizationInfoGraphQLPayload()))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"message":"unsupported"}]}`))
	}))
}

// RunPrivateRegistryAuthAndSettingsFinalize asserts .env has no private key, runs auth
// validation against a minimal GraphQL mock (getCreOrganizationInfo), then loads settings
// from disk and finalizes org-derived workflow owner for deployment-registry reg-test.
func RunPrivateRegistryAuthAndSettingsFinalize(t *testing.T, envPath, blankWorkflowDir string) {
	t.Helper()
	assertEnvFileHasNoEthPrivateKey(t, envPath)

	orgSrv := graphQLMockOrgInfoOnly(t)
	defer orgSrv.Close()
	t.Setenv(environments.EnvVarGraphQLURL, orgSrv.URL+"/graphql")

	bearerHome := CreateTestBearerCredentialsHome(t)
	t.Setenv("HOME", bearerHome)
	t.Setenv("USERPROFILE", bearerHome)

	logger := testutil.NewTestLogger()
	creds, err := credentials.New(logger)
	require.NoError(t, err)
	require.False(t, creds.IsValidated)

	envSet, err := environments.New()
	require.NoError(t, err)
	require.NotEmpty(t, envSet.GraphQLURL)

	val := authvalidation.NewValidator(creds, envSet, logger)
	result, err := val.ValidateCredentials(context.Background(), creds)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "test-org-id", result.OrgID)

	derivedFormatted, err := ethkeys.FormatWorkflowOwnerAddress(result.DerivedWorkflowOwner)
	require.NoError(t, err)
	wantDerived, err := ethkeys.FormatWorkflowOwnerAddress("0x" + MockDerivedWorkflowOwnerHex)
	require.NoError(t, err)
	require.Equal(t, wantDerived, derivedFormatted, "derived owner from mock GQL must match formatted MockDerivedWorkflowOwnerHex")

	restoreWD, err := testutil.ChangeWorkingDirectory(blankWorkflowDir)
	require.NoError(t, err)
	defer restoreWD()

	v := viper.New()
	settings.LoadEnv(logger, v, envPath)
	cmd := workflowDeployCmd()
	s, err := settings.New(logger, v, cmd, "")
	require.NoError(t, err)
	require.NotNil(t, s)
	require.Empty(t, s.User.EthPrivateKey, "CRE_ETH_PRIVATE_KEY must be absent")
	require.Equal(t, "reg-test", s.Workflow.UserWorkflowSettings.DeploymentRegistry)
	require.Empty(t, s.Workflow.UserWorkflowSettings.WorkflowOwnerAddress, "owner is deferred until finalize when deployment-registry is set")
	require.Empty(t, s.Workflow.UserWorkflowSettings.WorkflowOwnerType)

	tenantCtx := &tenantctx.EnvironmentContext{
		DefaultDonFamily: "test-don",
		Registries: []*tenantctx.Registry{
			{ID: "reg-test", Type: "OFF_CHAIN"},
		},
	}
	resolved, err := settings.ResolveRegistry("reg-test", tenantCtx, envSet)
	require.NoError(t, err)
	require.NotNil(t, resolved)
	require.Equal(t, settings.RegistryTypeOffChain, resolved.Type())

	err = settings.FinalizeWorkflowOwner(v, cmd, &s.Workflow, s.User.TargetName, resolved, derivedFormatted)
	require.NoError(t, err)
	require.Equal(t, derivedFormatted, s.Workflow.UserWorkflowSettings.WorkflowOwnerAddress)
	require.Equal(t, constants.WorkflowOwnerTypeOrgDerived, s.Workflow.UserWorkflowSettings.WorkflowOwnerType)
}

// RunPrivateRegistryE2E runs auth/settings finalize (no private key) then the full CLI
// private-registry lifecycle (deploy, pause, activate, delete) with httptest GraphQL mocks.
func RunPrivateRegistryE2E(t *testing.T, tc TestConfig, envPath, blankWorkflowDir string) {
	t.Helper()
	t.Run("auth_and_settings_finalize_without_private_key", func(t *testing.T) {
		RunPrivateRegistryAuthAndSettingsFinalize(t, envPath, blankWorkflowDir)
	})
	t.Run("cli_private_registry_lifecycle", func(t *testing.T) {
		RunWorkflowPrivateRegistryHappyPath(t, tc)
		RunWorkflowPausePrivateRegistryHappyPath(t, tc)
		RunWorkflowActivatePrivateRegistryHappyPath(t, tc)
		RunWorkflowDeletePrivateRegistryHappyPath(t, tc)
	})
}

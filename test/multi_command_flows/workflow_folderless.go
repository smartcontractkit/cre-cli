package multi_command_flows

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

// Folder-less workflow IDs/names used by the mock platform API.
const (
	folderlessWorkflowName       = "folderless-wf"
	folderlessWorkflowID         = "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	folderlessPausedWorkflowName = "folderless-wf-paused"
	folderlessPausedWorkflowID   = "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
)

// newFolderlessMockServer mocks the platform GraphQL API for folder-less
// pause/delete/activate: org info, tenant config (off-chain registry
// "reg-test"), the workflows list query, and the lifecycle mutations. Any
// other query fails.
func newFolderlessMockServer(t *testing.T, pauseCalled, deleteCalled, activateCalled *atomic.Bool) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/graphql") || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
			return
		}

		var req graphQLRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(req.Query, "reportUserEvent"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"reportUserEvent": map[string]any{"success": true, "message": ""},
				},
			})

		case strings.Contains(req.Query, "getCreOrganizationInfo"):
			_ = json.NewEncoder(w).Encode(mockGetCreOrganizationInfoGraphQLPayload())

		case strings.Contains(req.Query, "GetTenantConfig") || strings.Contains(req.Query, "getTenantConfig"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"getTenantConfig": map[string]any{
						"tenantId":         "42",
						"defaultDonFamily": "test-don",
						"vaultGatewayUrl":  "https://vault.example.test",
						"capabilitiesRegistry": map[string]any{
							"chainSelector": "6433500567565415381",
							"address":       "0x5FbDB2315678afecb367f032d93F642f64180aa3",
						},
						"registries": []map[string]any{
							{
								"id":    "reg-test",
								"label": "reg-test",
								"type":  "OFF_CHAIN",
							},
						},
						"forwarders": []any{},
					},
				},
			})

		case strings.Contains(req.Query, "ListWorkflows"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"workflows": map[string]any{
						"data": []map[string]any{
							{
								"uuid":           "9f2e7b6a-1111-4222-8333-444455556666",
								"name":           folderlessWorkflowName,
								"workflowId":     folderlessWorkflowID,
								"ownerAddress":   MockDerivedWorkflowOwnerHex, // raw, no 0x — as the platform returns it
								"status":         "ACTIVE",
								"workflowSource": "reg-test",
							},
							{
								"uuid":           "8f1e6b5a-2222-4333-8444-555566667777",
								"name":           folderlessPausedWorkflowName,
								"workflowId":     folderlessPausedWorkflowID,
								"ownerAddress":   MockDerivedWorkflowOwnerHex,
								"status":         "PAUSED",
								"workflowSource": "reg-test",
							},
						},
						"count": 2,
					},
				},
			})

		case strings.Contains(req.Query, "PauseOffchainWorkflow"):
			pauseCalled.Store(true)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"pauseOffchainWorkflow": map[string]any{
						"workflow": map[string]any{
							"workflowId":     folderlessWorkflowID,
							"owner":          privateRegistryOwnerAddress,
							"createdAt":      "2025-01-01T00:00:00Z",
							"status":         "WORKFLOW_STATUS_PAUSED",
							"workflowName":   folderlessWorkflowName,
							"binaryUrl":      "https://example.test/binary.wasm",
							"configUrl":      "",
							"tag":            folderlessWorkflowName,
							"attributes":     "",
							"donFamily":      "test-don",
							"organizationId": "test-org-id",
						},
					},
				},
			})

		case strings.Contains(req.Query, "ActivateOffchainWorkflow"):
			activateCalled.Store(true)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"activateOffchainWorkflow": map[string]any{
						"workflow": map[string]any{
							"workflowId":     folderlessPausedWorkflowID,
							"owner":          privateRegistryOwnerAddress,
							"createdAt":      "2025-01-01T00:00:00Z",
							"status":         "WORKFLOW_STATUS_ACTIVE",
							"workflowName":   folderlessPausedWorkflowName,
							"binaryUrl":      "https://example.test/binary.wasm",
							"configUrl":      "",
							"tag":            folderlessPausedWorkflowName,
							"attributes":     "",
							"donFamily":      "test-don",
							"organizationId": "test-org-id",
						},
					},
				},
			})

		case strings.Contains(req.Query, "DeleteOffchainWorkflow"):
			deleteCalled.Store(true)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"deleteOffchainWorkflow": map[string]any{
						"workflowId": folderlessWorkflowID,
					},
				},
			})

		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query: " + req.Query}},
			})
		}
	}))
}

// runFolderlessCLI runs the CLI from an empty directory (no project.yaml) so
// pause/delete must resolve the workflow remotely.
func runFolderlessCLI(t *testing.T, gqlURL string, args ...string) string {
	t.Helper()

	cmd := exec.Command(CLIPath, args...)
	cmd.Dir = t.TempDir() // no CRE project here on purpose
	cmd.Env = append(testutil.CLIChildEnv(t, CreateTestBearerCredentialsHome(t)),
		environments.EnvVarGraphQLURL+"="+gqlURL,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	require.NoError(
		t,
		cmd.Run(),
		"cre %s failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
		strings.Join(args, " "), stdout.String(), stderr.String(),
	)
	return StripANSI(stdout.String() + stderr.String())
}

// RunFolderlessPauseDeleteHappyPath pauses, activates, and deletes workflows
// without a project folder: by name, by on-chain ID, and with the picker-less
// default resolution path.
func RunFolderlessPauseDeleteHappyPath(t *testing.T) {
	t.Helper()

	var pauseCalled, deleteCalled, activateCalled atomic.Bool
	srv := newFolderlessMockServer(t, &pauseCalled, &deleteCalled, &activateCalled)
	defer srv.Close()
	gqlURL := srv.URL + "/graphql"

	// Pause by workflow name.
	out := runFolderlessCLI(t, gqlURL,
		"workflow", "pause", folderlessWorkflowName, "--"+settings.Flags.SkipConfirmation.Name)
	require.True(t, pauseCalled.Load(), "expected PauseOffchainWorkflow to be called")
	require.Contains(t, out, "Workflow paused successfully", "expected pause success.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Registry:         reg-test", "expected registry ID in details.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow Name:    "+folderlessWorkflowName, "expected workflow name in details.\nCLI OUTPUT:\n%s", out)

	// Pause by on-chain workflow ID.
	pauseCalled.Store(false)
	out = runFolderlessCLI(t, gqlURL,
		"workflow", "pause", folderlessWorkflowID, "--"+settings.Flags.SkipConfirmation.Name)
	require.True(t, pauseCalled.Load(), "expected PauseOffchainWorkflow to be called for ID resolution")
	require.Contains(t, out, "Workflow paused successfully", "expected pause success for ID resolution.\nCLI OUTPUT:\n%s", out)

	// Activate by workflow name (the paused workflow; the ACTIVE one must be
	// filtered out by the PAUSED-only candidate filter).
	out = runFolderlessCLI(t, gqlURL,
		"workflow", "activate", folderlessPausedWorkflowName, "--"+settings.Flags.SkipConfirmation.Name)
	require.True(t, activateCalled.Load(), "expected ActivateOffchainWorkflow to be called")
	require.Contains(t, out, "Workflow activated successfully", "expected activate success.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Workflow Name:    "+folderlessPausedWorkflowName, "expected workflow name in details.\nCLI OUTPUT:\n%s", out)

	// Delete by workflow name.
	out = runFolderlessCLI(t, gqlURL,
		"workflow", "delete", folderlessWorkflowName, "--"+settings.Flags.SkipConfirmation.Name)
	require.True(t, deleteCalled.Load(), "expected DeleteOffchainWorkflow to be called")
	require.Contains(t, out, "Workflows deleted successfully", "expected delete success.\nCLI OUTPUT:\n%s", out)
	require.Contains(t, out, "Deleted workflow ID: "+folderlessWorkflowID, "expected deleted workflow ID.\nCLI OUTPUT:\n%s", out)
}

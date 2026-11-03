package test

// Throwaway branch-verification tests for the folder-less pause/delete/activate
// feature (branch improve-delete-pause-wf). These complement
// multi_command_flows.RunFolderlessPauseDeleteHappyPath with error paths,
// scoping flags, routing decisions, and eligibility filters.

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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
	"github.com/smartcontractkit/cre-cli/test/multi_command_flows"
)

// ---- configurable mock platform ----

type folderlessRow struct {
	uuid     string
	name     string
	id       string
	owner    string
	status   string
	source   string
}

type folderlessMock struct {
	srv *httptest.Server
	// mutable rows: the platform indexer would reflect on-chain changes
	rows []folderlessRow
	// tenant registries served by GetTenantConfig
	registries []map[string]any
	// recorded mutation inputs
	pausedIDs    []string
	activatedIDs []string
	deletedIDs   []string
}

func (m *folderlessMock) setStatus(id, status string) {
	for i := range m.rows {
		if strings.EqualFold(m.rows[i].id, id) {
			m.rows[i].status = status
		}
	}
}

func newFolderlessMock(t *testing.T, rows []folderlessRow) *folderlessMock {
	t.Helper()
	return newFolderlessMockWithRegistries(t, rows, []map[string]any{
		{"id": "reg-test", "label": "reg-test", "type": "OFF_CHAIN"},
		{"id": "reg-other", "label": "reg-other", "type": "OFF_CHAIN"},
	})
}

func newFolderlessMockNoRegistries(t *testing.T, rows []folderlessRow) *folderlessMock {
	t.Helper()
	return newFolderlessMockWithRegistries(t, rows, []map[string]any{})
}

func newFolderlessMockWithRegistries(t *testing.T, rows []folderlessRow, registries []map[string]any) *folderlessMock {
	t.Helper()
	m := &folderlessMock{rows: rows, registries: registries}

	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/graphql") || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(req.Query, "reportUserEvent"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"reportUserEvent": map[string]any{"success": true, "message": ""}},
			})

		case strings.Contains(req.Query, "getCreOrganizationInfo"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"getCreOrganizationInfo": map[string]any{
						"orgId":                 "test-org-id",
						"derivedWorkflowOwners": []string{multi_command_flows.MockDerivedWorkflowOwnerHex},
					},
				},
			})

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
						"registries": m.registries,
						"forwarders": []any{},
					},
				},
			})

		case strings.Contains(req.Query, "ListWorkflows"):
			// emulate server-side filters: search (contains) + workflowOwnerAddress
			search, _ := req.Variables["input"].(map[string]any)["search"].(string)
			owners, _ := req.Variables["input"].(map[string]any)["workflowOwnerAddress"].([]any)

			var data []map[string]any
			for _, row := range m.rows {
				if search != "" && !strings.Contains(strings.ToLower(row.name), strings.ToLower(search)) {
					continue
				}
				if len(owners) > 0 {
					match := false
					for _, o := range owners {
						if os, ok := o.(string); ok && strings.EqualFold(strings.TrimPrefix(os, "0x"), row.owner) {
							match = true
						}
					}
					if !match {
						continue
					}
				}
				data = append(data, map[string]any{
					"uuid":           row.uuid,
					"name":           row.name,
					"workflowId":     row.id,
					"ownerAddress":   row.owner,
					"status":         row.status,
					"workflowSource": row.source,
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"workflows": map[string]any{"data": data, "count": len(data)},
				},
			})

		case strings.Contains(req.Query, "PauseOffchainWorkflow"):
			id, _ := req.Variables["request"].(map[string]any)["workflowId"].(string)
			m.pausedIDs = append(m.pausedIDs, id)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"pauseOffchainWorkflow": map[string]any{
						"workflow": map[string]any{
							"workflowId":     id,
							"owner":          "0x" + multi_command_flows.MockDerivedWorkflowOwnerHex,
							"createdAt":      "2025-01-01T00:00:00Z",
							"status":         "WORKFLOW_STATUS_PAUSED",
							"workflowName":   "wf",
							"binaryUrl":      "https://example.test/binary.wasm",
							"configUrl":      "",
							"tag":            "wf",
							"attributes":     "",
							"donFamily":      "test-don",
							"organizationId": "test-org-id",
						},
					},
				},
			})

		case strings.Contains(req.Query, "ActivateOffchainWorkflow"):
			id, _ := req.Variables["request"].(map[string]any)["workflowId"].(string)
			m.activatedIDs = append(m.activatedIDs, id)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"activateOffchainWorkflow": map[string]any{
						"workflow": map[string]any{
							"workflowId":     id,
							"owner":          "0x" + multi_command_flows.MockDerivedWorkflowOwnerHex,
							"createdAt":      "2025-01-01T00:00:00Z",
							"status":         "WORKFLOW_STATUS_ACTIVE",
							"workflowName":   "wf",
							"binaryUrl":      "https://example.test/binary.wasm",
							"configUrl":      "",
							"tag":            "wf",
							"attributes":     "",
							"donFamily":      "test-don",
							"organizationId": "test-org-id",
						},
					},
				},
			})

		case strings.Contains(req.Query, "DeleteOffchainWorkflow"):
			id, _ := req.Variables["request"].(map[string]any)["workflowId"].(string)
			m.deletedIDs = append(m.deletedIDs, id)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"deleteOffchainWorkflow": map[string]any{"workflowId": id},
				},
			})

		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query: " + req.Query}},
			})
		}
	}))
	t.Cleanup(m.srv.Close)
	return m
}

// runFolderless runs the CLI from an empty dir against the mock and returns
// combined output + error (no require on success).
func runFolderless(t *testing.T, gqlURL string, extraEnv []string, args ...string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, CLIPath, args...)
	cmd.Dir = t.TempDir() // no CRE project here on purpose
	cmd.Env = append(testutil.CLIChildEnv(t, multi_command_flows.CreateTestBearerCredentialsHome(t)),
		environments.EnvVarGraphQLURL+"="+gqlURL,
	)
	cmd.Env = append(cmd.Env, extraEnv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	return multi_command_flows.StripANSI(stdout.String() + stderr.String()), err
}

func folderlessTestRows() []folderlessRow {
	owner := multi_command_flows.MockDerivedWorkflowOwnerHex
	return []folderlessRow{
		{uuid: "9f2e7b6a-1111-4222-8333-444455556666", name: "wf-active", id: "1111111111111111111111111111111111111111111111111111111111111111", owner: owner, status: "ACTIVE", source: "reg-test"},
		{uuid: "8f1e6b5a-2222-4333-8444-555566667777", name: "wf-paused", id: "2222222222222222222222222222222222222222222222222222222222222222", owner: owner, status: "PAUSED", source: "reg-test"},
		{uuid: "7f0e5c4b-3333-4444-8555-666677778888", name: "wf-dup", id: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", owner: owner, status: "ACTIVE", source: "reg-test"},
		{uuid: "6e0d4c3b-4444-4555-8666-777788889999", name: "wf-dup", id: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", owner: owner, status: "ACTIVE", source: "reg-other"},
		{uuid: "5d0c3b2a-5555-4666-8777-888899990000", name: "wf-deleted", id: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", owner: owner, status: "DELETED", source: "reg-test"},
	}
}

func TestFolderlessCases(t *testing.T) {
	t.Setenv(environments.EnvVarEnv, "STAGING")
	t.Setenv(environments.EnvVarDonFamily, "test-don")

	mock := newFolderlessMock(t, folderlessTestRows())
	gqlURL := mock.srv.URL + "/graphql"

	yes := "--yes"

	// ---------- routing / arg validation ----------

	t.Run("TooManyArgs", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "a", "b")
		require.Error(t, err)
		require.Contains(t, out, "accepts at most 1 arg", out)
	})

	t.Run("PathLikeArgKeepsLegacyFlow", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "./definitely-missing", yes)
		require.Error(t, err)
		require.NotContains(t, out, `no workflow found with name "./definitely-missing"`, out)
	})

	t.Run("ExistingDirArgKeepsLegacyFlow", func(t *testing.T) {
		// create a dir named mywf inside an otherwise project-less cwd
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "mywf"), 0o755))

		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, CLIPath, "workflow", "pause", "mywf", yes)
		cmd.Dir = dir
		cmd.Env = append(testutil.CLIChildEnv(t, multi_command_flows.CreateTestBearerCredentialsHome(t)),
			environments.EnvVarGraphQLURL+"="+gqlURL,
		)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		out := multi_command_flows.StripANSI(stdout.String() + stderr.String())
		require.Error(t, err)
		require.NotContains(t, out, `no workflow found with name "mywf"`, out)
	})

	t.Run("NoArgNonInteractive", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "--non-interactive")
		require.Error(t, err)
		require.Contains(t, out, "a workflow name or ID is required in non-interactive mode", out)
	})

	t.Run("NoArgPickerWithoutTTYFails", func(t *testing.T) {
		// interactive picker requested but no TTY: must error, not hang
		// (runner has a 90s timeout; huh fails fast on non-TTY stdin)
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause")
		require.Error(t, err)
		require.NotContains(t, out, "a workflow name or ID is required in non-interactive mode", out)
	})

	// ---------- guards ----------

	t.Run("UnsignedGuard", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "wf-active", "--unsigned")
		require.Error(t, err)
		require.Contains(t, out, "--unsigned and --changeset require a project folder", out)
	})

	t.Run("ChangesetGuardOnDelete", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "delete", "wf-active", "--changeset")
		require.Error(t, err)
		require.Contains(t, out, "--unsigned and --changeset require a project folder", out)
	})

	t.Run("NonInteractiveWithoutYes", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "delete", "wf-active", "--non-interactive")
		require.Error(t, err)
		require.Contains(t, out, "missing required flags for --non-interactive mode", out)
	})

	// ---------- resolution errors ----------

	t.Run("NameNotFound", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "wf-unknown", yes)
		require.Error(t, err)
		require.Contains(t, out, `no workflow found with name "wf-unknown"`, out)
	})

	t.Run("IDNotFound", func(t *testing.T) {
		id := "0000000000000000000000000000000000000000000000000000000000000000"
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", id, yes)
		require.Error(t, err)
		require.Contains(t, out, `no workflow found with ID "`+id+`"`, out)
	})

	t.Run("UnknownRegistryFilter", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "wf-active", "--registry", "bogus", yes)
		require.Error(t, err)
		require.Contains(t, out, `registry "bogus" not found in user context`, out)
	})

	// ---------- eligibility filters ----------

	t.Run("PauseRejectsPausedByName", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "wf-paused", yes)
		require.Error(t, err)
		require.Contains(t, out, "none of its deployments are eligible", out)
	})

	t.Run("PauseRejectsPausedByID", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "2222222222222222222222222222222222222222222222222222222222222222", yes)
		require.Error(t, err)
		require.Contains(t, out, "is not eligible for this operation", out)
	})

	t.Run("ActivateRejectsActiveByName", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "activate", "wf-active", yes)
		require.Error(t, err)
		require.Contains(t, out, "none of its deployments are eligible", out)
	})

	t.Run("DeletedWorkflowNotEligible", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "wf-deleted", yes)
		require.Error(t, err)
		require.Contains(t, out, "none of its deployments are eligible", out)
	})

	// ---------- scoping ----------

	t.Run("AmbiguousNameNonInteractive", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "wf-dup", "--non-interactive", yes)
		require.Error(t, err)
		require.Contains(t, out, `multiple workflows named "wf-dup" found`, out)
	})

	t.Run("RegistryScopeDisambiguates", func(t *testing.T) {
		before := len(mock.pausedIDs)
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "wf-dup", "--registry", "reg-other", yes)
		require.NoError(t, err, out)
		require.Contains(t, out, "Workflow paused successfully", out)
		require.Len(t, mock.pausedIDs, before+1)
		require.Equal(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", mock.pausedIDs[len(mock.pausedIDs)-1])
	})

	t.Run("OwnerScopeNoMatch", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "wf-active",
			"--owner", "0x000000000000000000000000000000000000dEaD", yes)
		require.Error(t, err)
		require.Contains(t, out, `no workflow found with name "wf-active"`, out)
	})

	t.Run("OwnerScopeMatch", func(t *testing.T) {
		out, err := runFolderless(t, gqlURL, nil, "workflow", "pause", "wf-active",
			"--owner", "0x"+multi_command_flows.MockDerivedWorkflowOwnerHex, yes)
		require.NoError(t, err, out)
		require.Contains(t, out, "Workflow paused successfully", out)
	})

	// ---------- happy-path extras ----------

	t.Run("DeletePausedByName", func(t *testing.T) {
		before := len(mock.deletedIDs)
		out, err := runFolderless(t, gqlURL, nil, "workflow", "delete", "wf-paused", yes)
		require.NoError(t, err, out)
		require.Contains(t, out, "Workflows deleted successfully", out)
		require.Len(t, mock.deletedIDs, before+1)
		require.Equal(t, "2222222222222222222222222222222222222222222222222222222222222222", mock.deletedIDs[len(mock.deletedIDs)-1])
	})

	t.Run("ActivateByID", func(t *testing.T) {
		before := len(mock.activatedIDs)
		out, err := runFolderless(t, gqlURL, nil, "workflow", "activate", "2222222222222222222222222222222222222222222222222222222222222222", yes)
		require.NoError(t, err, out)
		require.Contains(t, out, "Workflow activated successfully", out)
		require.Len(t, mock.activatedIDs, before+1)
		require.Equal(t, "2222222222222222222222222222222222222222222222222222222222222222", mock.activatedIDs[len(mock.activatedIDs)-1])
	})
}

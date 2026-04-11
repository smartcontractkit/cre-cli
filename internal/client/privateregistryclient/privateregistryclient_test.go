package privateregistryclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func TestValidateUpsertWorkflowInput(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		err := validateUpsertWorkflowInput(WorkflowInRegistryInput{
			WorkflowID:   "wf-1",
			Status:       WorkflowStatusActive,
			WorkflowName: "test-workflow",
			BinaryURL:    "s3://binary",
			DonFamily:    "family-a",
		})
		require.NoError(t, err)
	})

	tests := []struct {
		name  string
		input WorkflowInRegistryInput
		err   string
	}{
		{
			name:  "missing workflowId",
			input: WorkflowInRegistryInput{Status: WorkflowStatusActive, WorkflowName: "w", BinaryURL: "b", DonFamily: "f"},
			err:   "workflowId is required",
		},
		{
			name:  "missing status",
			input: WorkflowInRegistryInput{WorkflowID: "wf", WorkflowName: "w", BinaryURL: "b", DonFamily: "f"},
			err:   "status is required",
		},
		{
			name:  "missing workflowName",
			input: WorkflowInRegistryInput{WorkflowID: "wf", Status: WorkflowStatusActive, BinaryURL: "b", DonFamily: "f"},
			err:   "workflowName is required",
		},
		{
			name:  "missing binaryUrl",
			input: WorkflowInRegistryInput{WorkflowID: "wf", Status: WorkflowStatusActive, WorkflowName: "w", DonFamily: "f"},
			err:   "binaryUrl is required",
		},
		{
			name:  "missing donFamily",
			input: WorkflowInRegistryInput{WorkflowID: "wf", Status: WorkflowStatusActive, WorkflowName: "w", BinaryURL: "b"},
			err:   "donFamily is required",
		},
		{
			name:  "invalid status",
			input: WorkflowInRegistryInput{WorkflowID: "wf", Status: "INVALID", WorkflowName: "w", BinaryURL: "b", DonFamily: "f"},
			err:   "status must be one of \"WORKFLOW_STATUS_UNSPECIFIED\", \"WORKFLOW_STATUS_ACTIVE\", \"WORKFLOW_STATUS_PAUSED\"",
		},
		{
			name:  "workflowName exceeds max length",
			input: WorkflowInRegistryInput{WorkflowID: "wf", Status: WorkflowStatusActive, WorkflowName: strings.Repeat("a", maxWorkflowNameLength+1), BinaryURL: "b", DonFamily: "f"},
			err:   "workflowName exceeds max length 64",
		},
		{
			name:  "binaryUrl exceeds max length",
			input: WorkflowInRegistryInput{WorkflowID: "wf", Status: WorkflowStatusActive, WorkflowName: "w", BinaryURL: strings.Repeat("b", maxBinaryURLLength+1), DonFamily: "f"},
			err:   "binaryUrl exceeds max length 200",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUpsertWorkflowInput(tc.input)
			require.Error(t, err)
			assert.Equal(t, tc.err, err.Error())
		})
	}
}

func TestUpsertWorkflowInRegistry(t *testing.T) {
	var capturedQuery string
	var capturedVariables map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedQuery = req.Query
		capturedVariables = req.Variables

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"upsertWorkflowInRegistry": map[string]any{
					"workflow": map[string]any{
						"workflowId":     "wf-123",
						"owner":          "owner-1",
						"createdAt":      "2026-01-01T00:00:00Z",
						"status":         "WORKFLOW_STATUS_ACTIVE",
						"workflowName":   "registry-workflow",
						"binaryUrl":      "s3://binary",
						"configUrl":      "s3://config",
						"tag":            "v1",
						"attributes":     "{\"region\":\"us-east-1\"}",
						"donFamily":      "family-a",
						"organizationId": "org-1",
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestPrivateRegistryClient(t, srv.URL)
	configURL := "s3://config"
	tag := "v1"
	attributes := "{\"region\":\"us-east-1\"}"
	result, err := client.UpsertWorkflowInRegistry(WorkflowInRegistryInput{
		WorkflowID:   "wf-123",
		Status:       WorkflowStatusActive,
		WorkflowName: "registry-workflow",
		BinaryURL:    "s3://binary",
		ConfigURL:    &configURL,
		Tag:          &tag,
		Attributes:   &attributes,
		DonFamily:    "family-a",
	})

	require.NoError(t, err)
	assert.Contains(t, capturedQuery, "mutation UpsertWorkflowInRegistry")
	assert.Contains(t, capturedQuery, "upsertWorkflowInRegistry")
	assert.Equal(t, "wf-123", result.WorkflowID)
	assert.Equal(t, WorkflowStatusActive, result.Status)
	assert.Equal(t, "family-a", result.DonFamily)
	assert.Equal(t, "org-1", result.OrganizationID)

	request, ok := capturedVariables["request"].(map[string]any)
	require.True(t, ok)
	workflow, ok := request["workflow"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "wf-123", workflow["workflowId"])
	assert.Equal(t, "WORKFLOW_STATUS_ACTIVE", workflow["status"])
	assert.Equal(t, "registry-workflow", workflow["workflowName"])
	assert.Equal(t, "s3://binary", workflow["binaryUrl"])
	assert.Equal(t, "family-a", workflow["donFamily"])
}

func TestUpsertWorkflowInRegistry_GQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "upsert failed"}},
		})
	}))
	defer srv.Close()

	client := newTestPrivateRegistryClient(t, srv.URL)
	_, err := client.UpsertWorkflowInRegistry(WorkflowInRegistryInput{
		WorkflowID:   "wf-123",
		Status:       WorkflowStatusActive,
		WorkflowName: "registry-workflow",
		BinaryURL:    "s3://binary",
		DonFamily:    "family-a",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upsert workflow in registry")
}

func TestPauseWorkflowInRegistry(t *testing.T) {
	var capturedVariables map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedVariables = req.Variables

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"pauseWorkflowInRegistry": map[string]any{
					"workflow": map[string]any{
						"workflowId":     "wf-123",
						"status":         "WORKFLOW_STATUS_PAUSED",
						"workflowName":   "registry-workflow",
						"binaryUrl":      "s3://binary",
						"donFamily":      "family-a",
						"organizationId": "org-1",
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestPrivateRegistryClient(t, srv.URL)
	result, err := client.PauseWorkflowInRegistry("wf-123")
	require.NoError(t, err)
	assert.Equal(t, "wf-123", result.WorkflowID)
	assert.Equal(t, WorkflowStatusPaused, result.Status)

	request, ok := capturedVariables["request"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "wf-123", request["workflowId"])
}

func TestActivateWorkflowInRegistry(t *testing.T) {
	var capturedVariables map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]interface{} `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedVariables = req.Variables

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"activateWorkflowInRegistry": map[string]any{
					"workflow": map[string]any{
						"workflowId":     "wf-123",
						"status":         "WORKFLOW_STATUS_ACTIVE",
						"workflowName":   "registry-workflow",
						"binaryUrl":      "s3://binary",
						"donFamily":      "family-a",
						"organizationId": "org-1",
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestPrivateRegistryClient(t, srv.URL)
	result, err := client.ActivateWorkflowInRegistry("wf-123")
	require.NoError(t, err)
	assert.Equal(t, WorkflowStatusActive, result.Status)

	request, ok := capturedVariables["request"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "wf-123", request["workflowId"])
}

func TestDeleteWorkflowInRegistry(t *testing.T) {
	var capturedVariables map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]interface{} `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		capturedVariables = req.Variables

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"deleteWorkflowInRegistry": map[string]any{
					"workflowId": "wf-123",
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestPrivateRegistryClient(t, srv.URL)
	deletedWorkflowID, err := client.DeleteWorkflowInRegistry("wf-123")
	require.NoError(t, err)
	assert.Equal(t, "wf-123", deletedWorkflowID)

	request, ok := capturedVariables["request"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "wf-123", request["workflowId"])
}

func TestWorkflowMutations_RequireWorkflowID(t *testing.T) {
	logger := testutil.NewTestLogger()
	client := New(nil, logger)

	_, pauseErr := client.PauseWorkflowInRegistry("")
	require.EqualError(t, pauseErr, "workflowId is required")

	_, activateErr := client.ActivateWorkflowInRegistry("")
	require.EqualError(t, activateErr, "workflowId is required")

	_, deleteErr := client.DeleteWorkflowInRegistry("")
	require.EqualError(t, deleteErr, "workflowId is required")
}

func newTestPrivateRegistryClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	logger := testutil.NewTestLogger()
	creds := &credentials.Credentials{
		AuthType: credentials.AuthTypeApiKey,
		APIKey:   "test-api-key",
	}
	envSet := &environments.EnvironmentSet{GraphQLURL: serverURL}
	gqlClient := graphqlclient.New(creds, envSet, logger)
	return New(gqlClient, logger)
}

func TestCreateServiceContextWithTimeout(t *testing.T) {
	logger := zerolog.Nop()
	client := New(nil, &logger)
	client.SetServiceTimeout(150 * time.Millisecond)

	ctx, cancel := client.CreateServiceContextWithTimeout()
	defer cancel()

	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(150*time.Millisecond), deadline, 100*time.Millisecond)
}

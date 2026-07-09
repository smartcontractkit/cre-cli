package workflowdataclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetWorkflowSummary_MapsFields(t *testing.T) {
	registered := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	executed := time.Date(2026, 1, 15, 8, 30, 0, 0, time.UTC)
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		input, _ := vars["input"].(map[string]any)
		assert.Equal(t, "wf-uuid-1", input["uuid"])
		assert.Equal(t, from.UTC().Format(time.RFC3339), input["from"])

		gqlData(w, map[string]any{
			"workflow": map[string]any{
				"data": map[string]any{
					"uuid":           "wf-uuid-1",
					"name":           "my-workflow",
					"workflowId":     "abc123onchain",
					"ownerAddress":   "0xowner",
					"status":         "ACTIVE",
					"workflowSource": "private",
					"registeredAt":   registered.Format(time.RFC3339),
					"executedAt":     executed.Format(time.RFC3339),
					"executionCount": 10,
					"executionCountByStatus": map[string]any{
						"success": 8,
						"failure": 2,
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	got, err := client.GetWorkflowSummary(context.Background(), "wf-uuid-1", from)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "my-workflow", got.Name)
	assert.Equal(t, 10, got.ExecutionCount)
	assert.Equal(t, 8, got.SuccessCount)
	assert.Equal(t, 2, got.FailureCount)
	require.NotNil(t, got.ExecutedAt)
}

func TestGetWorkflowSummary_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlData(w, map[string]any{
			"workflow": map[string]any{"data": nil},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	_, err := client.GetWorkflowSummary(context.Background(), "missing", time.Now().UTC())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `workflow "missing" not found`)
}

func TestGetLatestDeployment_ReturnsMostRecent(t *testing.T) {
	deployed := time.Date(2026, 1, 10, 11, 55, 0, 0, time.UTC)
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	txHash := "0xabc"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		input, _ := vars["input"].(map[string]any)
		assert.Equal(t, "wf-uuid-1", input["workflowUUID"])
		assert.Equal(t, from.UTC().Format(time.RFC3339), input["from"])
		assert.Equal(t, to.UTC().Format(time.RFC3339), input["to"])

		gqlData(w, map[string]any{
			"workflowDeployments": map[string]any{
				"data": []any{
					map[string]any{
						"uuid":       "dep-uuid-1",
						"workflowID": "abc123onchain",
						"status":     "SUCCESS",
						"deployedAt": deployed.Format(time.RFC3339),
						"txHash":     txHash,
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	got, err := client.GetLatestDeployment(context.Background(), "wf-uuid-1", from, to)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "SUCCESS", got.Status)
	require.NotNil(t, got.TxHash)
	assert.Equal(t, txHash, *got.TxHash)
}

func TestGetLatestDeployment_EmptyReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlData(w, map[string]any{
			"workflowDeployments": map[string]any{"data": []any{}},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	got, err := client.GetLatestDeployment(context.Background(), "wf-uuid-1", time.Now().UTC(), time.Now().UTC())
	require.NoError(t, err)
	assert.Nil(t, got)
}

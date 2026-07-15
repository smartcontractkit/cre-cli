package workflowdataclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gqlData(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": payload})
}

func TestListExecutions_DefaultLimitAndMapping(t *testing.T) {
	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)
	finished := time.Date(2026, 5, 29, 14, 0, 17, 0, time.UTC)
	credit := "0.05"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		input, _ := vars["input"].(map[string]any)
		page, _ := input["page"].(map[string]any)
		assert.Equal(t, float64(20), page["size"])

		gqlData(w, map[string]any{
			"workflowExecutions": map[string]any{
				"count": 1,
				"data": []any{
					map[string]any{
						"uuid":         "exec-uuid-1",
						"id":           "0xabc",
						"workflowUUID": "wf-uuid-1",
						"workflowId":   "wf-onchain",
						"workflowName": "my-workflow",
						"status":       "FAILURE",
						"startedAt":    started.Format(time.RFC3339),
						"finishedAt":   finished.Format(time.RFC3339),
						"creditUsed":   credit,
						"errors": []any{
							map[string]any{"error": "boom", "count": 1},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	got, err := client.ListExecutions(context.Background(), ListExecutionsInput{})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "exec-uuid-1", got[0].UUID)
	assert.Equal(t, ExecutionStatusFailure, got[0].Status)
	require.NotNil(t, got[0].CreditUsed)
	assert.Equal(t, credit, *got[0].CreditUsed)
	require.Len(t, got[0].Errors, 1)
	assert.Equal(t, "boom", got[0].Errors[0].Error)
}

func TestListExecutions_PassesFilters(t *testing.T) {
	wfUUID := "wf-uuid-1"
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	search := "0xdeadbeef"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		input, _ := vars["input"].(map[string]any)
		assert.Equal(t, wfUUID, input["workflowUuid"])
		assert.Equal(t, search, input["search"])
		assert.Equal(t, from.UTC().Format(time.RFC3339), input["from"])
		assert.Equal(t, to.UTC().Format(time.RFC3339), input["to"])
		statuses, _ := input["status"].([]any)
		require.Len(t, statuses, 1)
		assert.Equal(t, "SUCCESS", statuses[0])

		gqlData(w, map[string]any{
			"workflowExecutions": map[string]any{"count": 0, "data": []any{}},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	_, err := client.ListExecutions(context.Background(), ListExecutionsInput{
		WorkflowUUID: &wfUUID,
		Statuses:     []ExecutionStatus{ExecutionStatusSuccess},
		From:         &from,
		To:           &to,
		Search:       &search,
		Limit:        50,
	})
	require.NoError(t, err)
}

func TestGetExecution_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlData(w, map[string]any{
			"workflowExecution": map[string]any{"data": nil},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	_, err := client.GetExecution(context.Background(), "missing-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `execution "missing-uuid" not found`)
}

func TestFindExecutionByOnChainID_ResolvesViaSearch(t *testing.T) {
	onChainID := "0xabc123"
	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query, _ := body["query"].(string)

		if strings.Contains(query, "ListExecutions") {
			gqlData(w, map[string]any{
				"workflowExecutions": map[string]any{
					"count": 1,
					"data": []any{
						map[string]any{
							"uuid": "exec-uuid-1", "id": onChainID,
							"workflowUUID": "wf-1", "workflowName": "wf",
							"status": "SUCCESS", "startedAt": started.Format(time.RFC3339),
							"errors": []any{},
						},
					},
				},
			})
			return
		}

		gqlData(w, map[string]any{
			"workflowExecution": map[string]any{
				"data": map[string]any{
					"uuid": "exec-uuid-1", "id": onChainID,
					"workflowUUID": "wf-1", "workflowName": "wf",
					"status": "SUCCESS", "startedAt": started.Format(time.RFC3339),
					"errors": []any{},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	got, err := client.FindExecutionByOnChainID(context.Background(), onChainID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "exec-uuid-1", got.UUID)
	assert.Equal(t, onChainID, got.ID)
}

func TestCountExecutions_ReturnsTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlData(w, map[string]any{
			"workflowExecutions": map[string]any{
				"count": 42,
				"data":  []any{},
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	count, err := client.CountExecutions(context.Background(), "wf-uuid-1", []ExecutionStatus{ExecutionStatusFailure})
	require.NoError(t, err)
	assert.Equal(t, 42, count)
}

func TestListExecutionEvents_PassesFilters(t *testing.T) {
	started := time.Date(2026, 5, 29, 14, 0, 5, 0, time.UTC)
	capID := "fetch-price"
	status := "FAILURE"
	method := "invoke"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		input, _ := vars["input"].(map[string]any)
		assert.Equal(t, "exec-uuid-1", input["workflowExecutionUUID"])
		assert.Equal(t, capID, input["capabilityID"])
		assert.Equal(t, status, input["status"])

		gqlData(w, map[string]any{
			"workflowExecutionEvents": map[string]any{
				"data": []any{
					map[string]any{
						"capabilityID": capID,
						"status":       status,
						"startedAt":    started.Format(time.RFC3339),
						"method":       method,
						"errors":       []any{},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	got, err := client.ListExecutionEvents(context.Background(), ListEventsInput{
		ExecutionUUID: "exec-uuid-1",
		CapabilityID:  &capID,
		Status:        &status,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, capID, got[0].CapabilityID)
	require.NotNil(t, got[0].Method)
	assert.Equal(t, method, *got[0].Method)
}

func TestListExecutionLogs_MapsRows(t *testing.T) {
	ts := time.Date(2026, 5, 29, 14, 0, 10, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		vars, _ := body["variables"].(map[string]any)
		input, _ := vars["input"].(map[string]any)
		assert.Equal(t, "exec-uuid-1", input["workflowExecutionUUID"])

		gqlData(w, map[string]any{
			"workflowExecutionLogs": map[string]any{
				"data": []any{
					map[string]any{
						"nodeID":    "ProcessData",
						"message":   "done",
						"timestamp": ts.Format(time.RFC3339),
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	got, err := client.ListExecutionLogs(context.Background(), "exec-uuid-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "ProcessData", got[0].NodeID)
	assert.Equal(t, "done", got[0].Message)
}

package workflowdataclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func newTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	logger := testutil.NewTestLogger()
	creds := &credentials.Credentials{
		AuthType: credentials.AuthTypeApiKey,
		APIKey:   "test-api-key",
	}
	envSet := &environments.EnvironmentSet{GraphQLURL: serverURL}
	gql := graphqlclient.New(creds, envSet, logger)
	return New(gql, logger)
}

func TestListAll_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"workflows": map[string]any{
					"count": 2,
					"data": []map[string]string{
						{
							"name":           "alpha",
							"workflowId":     "1010101010101010101010101010101010101010101010101010101010101010",
							"ownerAddress":   "2020202020202020202020202020202020202020",
							"status":         "ACTIVE",
							"workflowSource": "private",
						},
						{
							"name":           "beta",
							"workflowId":     "3030303030303030303030303030303030303030303030303030303030303030",
							"ownerAddress":   "4040404040404040404040404040404040404040",
							"status":         "PAUSED",
							"workflowSource": "contract:999888777666555444333:0xabababababababababababababababababababab",
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	got, err := client.ListAll(context.Background(), DefaultPageSize)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "alpha", got[0].Name)
	assert.Equal(t, "ACTIVE", got[0].Status)
	assert.Equal(t, "private", got[0].WorkflowSource)
	assert.Equal(t, "beta", got[1].Name)
	assert.Equal(t, "PAUSED", got[1].Status)
}

func TestListAll_Pagination(t *testing.T) {
	var callCount atomic.Int32

	page1Data := make([]map[string]string, DefaultPageSize)
	for i := range page1Data {
		page1Data[i] = map[string]string{
			"name":           "wf-page-1",
			"workflowId":     "a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0",
			"ownerAddress":   "b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0",
			"status":         "ACTIVE",
			"workflowSource": "private",
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := int(callCount.Add(1))
		w.Header().Set("Content-Type", "application/json")

		switch call {
		case 1:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"workflows": map[string]any{
						"count": DefaultPageSize + 1,
						"data":  page1Data,
					},
				},
			})
		case 2:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"workflows": map[string]any{
						"count": DefaultPageSize + 1,
						"data": []map[string]string{
							{
								"name":           "wf-last",
								"workflowId":     "c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0",
								"ownerAddress":   "b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0",
								"status":         "ACTIVE",
								"workflowSource": "private",
							},
						},
					},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"workflows": map[string]any{"count": DefaultPageSize + 1, "data": []any{}},
				},
			})
		}
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	got, err := client.ListAll(context.Background(), DefaultPageSize)
	require.NoError(t, err)
	assert.Len(t, got, DefaultPageSize+1)
	assert.Equal(t, "wf-last", got[len(got)-1].Name)
	assert.Equal(t, int32(2), callCount.Load(), "expected exactly 2 HTTP calls for 2 pages")
}

func TestListAll_GQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "unauthorized"}},
		})
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	_, err := client.ListAll(context.Background(), DefaultPageSize)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list workflows")
	assert.Contains(t, err.Error(), "unauthorized")
}

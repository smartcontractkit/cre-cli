package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/environments"
)

// NewGraphQLMockServerGetOrganization starts an httptest.Server that responds to
// getOrganization with a fixed organizationId. It sets EnvVarGraphQLURL so CLI
// commands use this server. Caller must defer srv.Close().
func NewGraphQLMockServerGetOrganization(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost {
			var req struct {
				Query     string                 `json:"query"`
				Variables map[string]interface{} `json:"variables"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(req.Query, "getOrganization") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getOrganization": map[string]any{"organizationId": "test-org-id"},
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
	t.Setenv(environments.EnvVarGraphQLURL, srv.URL+"/graphql")
	return srv
}

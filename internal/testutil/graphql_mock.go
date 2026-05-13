package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/environments"
)

// RespondGetTenantConfigMock writes mock getTenantConfig (with defaultDonFamily) when the query matches; returns whether it handled the request.
func RespondGetTenantConfigMock(w http.ResponseWriter, query string) bool {
	if !strings.Contains(query, "getTenantConfig") && !strings.Contains(query, "GetTenantConfig") {
		return false
	}
	_ = json.NewEncoder(w).Encode(mockGetTenantConfigData())
	return true
}

func mockGetTenantConfigData() map[string]any {
	return map[string]any{
		"data": map[string]any{
			"getTenantConfig": map[string]any{
				"tenantId":         "test-tenant",
				"defaultDonFamily": "test-don",
				"vaultGatewayUrl":  "https://vault.mock.invalid",
				"registries":       []map[string]any{},
			},
		},
	}
}

// NewGraphQLMockServerGetOrganization starts an httptest.Server that responds to
// getCreOrganizationInfo with a fixed orgId and derivedWorkflowOwners.
// It sets EnvVarGraphQLURL so CLI commands use this server. Caller must defer srv.Close().
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
			if RespondGetTenantConfigMock(w, req.Query) {
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

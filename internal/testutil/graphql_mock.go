package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/environments"
)

// MockGetCreOrganizationInfoGraphQLPayload returns a GraphQL response for getCreOrganizationInfo.
func MockGetCreOrganizationInfoGraphQLPayload() map[string]any {
	return map[string]any{
		"data": map[string]any{
			"getCreOrganizationInfo": map[string]any{
				"orgId":                 "test-org-id",
				"derivedWorkflowOwners": []string{"ab12cd34ef56ab12cd34ef56ab12cd34ef56ab12"},
			},
		},
	}
}

// QueryIsGetTenantConfig reports whether q is a getTenantConfig GraphQL operation.
func QueryIsGetTenantConfig(q string) bool {
	return strings.Contains(q, "GetTenantConfig") || strings.Contains(q, "getTenantConfig")
}

// MockGetTenantConfigGraphQLPayload returns a GraphQL response for getTenantConfig
// suitable for E2E tests using the anvil-devnet workflow registry defaults.
func MockGetTenantConfigGraphQLPayload() map[string]any {
	return map[string]any{
		"data": map[string]any{
			"getTenantConfig": map[string]any{
				"tenantId":         "test-tenant-id",
				"defaultDonFamily": "test-don",
				"vaultGatewayUrl":  "https://vault.example.test",
				"capabilitiesRegistry": map[string]any{
					"chainSelector": "6433500567565415381",
					"address":       "0x76c9cf548b4179F8901cda1f8623568b58215E62",
				},
				"registries": []map[string]any{
					{
						"id":               "anvil-devnet",
						"label":            "anvil-devnet",
						"type":             "ON_CHAIN",
						"chainSelector":    "6433500567565415381",
						"address":          "0x5FbDB2315678afecb367f032d93F642f64180aa3",
						"secretsAuthFlows": []string{"OWNER_KEY_SIGNING"},
					},
					{
						"id":               "private",
						"label":            "Private (Chainlink-hosted)",
						"type":             "OFF_CHAIN",
						"secretsAuthFlows": []string{"BROWSER"},
					},
				},
				"forwarders": []any{},
			},
		},
	}
}

// NewGraphQLMockServerGetOrganization starts an httptest.Server that responds to
// getCreOrganizationInfo and getTenantConfig with fixed test payloads.
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
				_ = json.NewEncoder(w).Encode(MockGetCreOrganizationInfoGraphQLPayload())
				return
			}
			if QueryIsGetTenantConfig(req.Query) {
				_ = json.NewEncoder(w).Encode(MockGetTenantConfigGraphQLPayload())
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

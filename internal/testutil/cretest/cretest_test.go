package cretest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/creconfig"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

// mockTenantConfigPayload matches test GraphQL mock data used by E2E tests.
func mockTenantConfigPayload() map[string]any {
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

// TestFetchAndWriteContext_DoesNotModifyRealHomeConfig ensures tenant context is written
// only under CRE_CONFIG_DIR, not the developer's ~/.cre directory.
func TestFetchAndWriteContext_DoesNotModifyRealHomeConfig(t *testing.T) {
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot resolve home dir: %v", err)
	}
	realContextPath := filepath.Join(realHome, creconfig.Dir, tenantctx.ContextFile)

	var before []byte
	var beforeStat os.FileInfo
	if st, statErr := os.Stat(realContextPath); statErr == nil {
		beforeStat = st
		before, _ = os.ReadFile(realContextPath)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockTenantConfigPayload())
	}))
	defer srv.Close()

	IsolateConfig(t)
	PinGoCacheForProcess(t)

	l := zerolog.Nop()
	log := &l
	client := graphqlclient.New(
		&credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"},
		&environments.EnvironmentSet{GraphQLURL: srv.URL},
		log,
	)

	if err := tenantctx.FetchAndWriteContext(context.Background(), client, "STAGING", log); err != nil {
		t.Fatalf("FetchAndWriteContext: %v", err)
	}

	isolatedPath, err := creconfig.FilePath(tenantctx.ContextFile)
	if err != nil {
		t.Fatalf("isolated context path: %v", err)
	}
	isolatedData, err := os.ReadFile(isolatedPath)
	if err != nil {
		t.Fatalf("read isolated context: %v", err)
	}
	isolated := string(isolatedData)
	if !strings.Contains(isolated, "test-tenant-id") || !strings.Contains(isolated, "anvil-devnet") {
		t.Fatalf("isolated context missing mock payload: %s", isolated)
	}

	afterStat, statErr := os.Stat(realContextPath)
	if beforeStat == nil {
		if statErr == nil {
			t.Fatalf("real %s was created during test", realContextPath)
		}
		return
	}
	if statErr != nil {
		t.Fatalf("real %s disappeared during test", realContextPath)
	}
	if !afterStat.ModTime().Equal(beforeStat.ModTime()) {
		t.Fatalf("real %s mtime changed during test", realContextPath)
	}
	after, err := os.ReadFile(realContextPath)
	if err != nil {
		t.Fatalf("read real context after test: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("real %s content changed during test", realContextPath)
	}
}

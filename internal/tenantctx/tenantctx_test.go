package tenantctx

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func newMockGQLServer(t *testing.T, response map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
}

func newCountingGQLServer(t *testing.T, counter *atomic.Int32, response map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
}

func gqlResponseOnChainAndPrivate() map[string]any {
	return map[string]any{
		"data": map[string]any{
			"getTenantConfig": map[string]any{
				"tenantId":         "42",
				"defaultDonFamily": "zone-a",
				"vaultGatewayUrl":  "https://gateway.example.com/",
				"registries": []any{
					map[string]any{
						"id":               "ethereum-testnet-sepolia",
						"label":            "ethereum-testnet-sepolia",
						"type":             "ON_CHAIN",
						"chainSelector":    "16015286601757825753",
						"address":          "0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135",
						"secretsAuthFlows": []any{"BROWSER", "OWNER_KEY_SIGNING"},
					},
					map[string]any{
						"id":               "private",
						"label":            "Private (Chainlink-hosted)",
						"type":             "OFF_CHAIN",
						"secretsAuthFlows": []any{"BROWSER"},
					},
				},
			},
		},
	}
}

func gqlResponsePrivateOnly() map[string]any {
	return map[string]any{
		"data": map[string]any{
			"getTenantConfig": map[string]any{
				"tenantId":         "99",
				"defaultDonFamily": "zone-b",
				"vaultGatewayUrl":  "https://gateway-private.example.com/",
				"registries": []any{
					map[string]any{
						"id":               "private",
						"label":            "Private (Chainlink-hosted)",
						"type":             "OFF_CHAIN",
						"secretsAuthFlows": []any{"BROWSER"},
					},
				},
			},
		},
	}
}

func newGQLClient(t *testing.T, serverURL string) *graphqlclient.Client {
	t.Helper()
	log := testutil.NewTestLogger()
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"}
	envSet := &environments.EnvironmentSet{GraphQLURL: serverURL}
	return graphqlclient.New(creds, envSet, log)
}

func fakeJWT(t *testing.T) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, _ := json.Marshal(map[string]any{"exp": time.Now().Add(time.Hour).Unix()})
	return fmt.Sprintf("%s.%s.sig", header, base64.RawURLEncoding.EncodeToString(payload))
}

// --- FetchAndWriteContext ---

func TestFetchAndWriteContext_OnChainAndPrivate(t *testing.T) {
	srv := newMockGQLServer(t, gqlResponseOnChainAndPrivate())
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	log := testutil.NewTestLogger()
	client := newGQLClient(t, srv.URL)

	err := FetchAndWriteContext(context.Background(), client, "STAGING", log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envCtx, err := LoadContext("STAGING")
	if err != nil {
		t.Fatalf("failed to load written context: %v", err)
	}

	if envCtx.TenantID != "42" {
		t.Errorf("TenantID = %q, want %q", envCtx.TenantID, "42")
	}
	if envCtx.DefaultDonFamily != "zone-a" {
		t.Errorf("DefaultDonFamily = %q, want %q", envCtx.DefaultDonFamily, "zone-a")
	}
	if envCtx.VaultGatewayURL != "https://gateway.example.com/" {
		t.Errorf("VaultGatewayURL = %q, want %q", envCtx.VaultGatewayURL, "https://gateway.example.com/")
	}
	if len(envCtx.Registries) != 2 {
		t.Fatalf("expected 2 registries, got %d", len(envCtx.Registries))
	}

	onchain := envCtx.Registries[0]
	if onchain.ID != "onchain:ethereum-testnet-sepolia" {
		t.Errorf("on-chain ID = %q, want %q", onchain.ID, "onchain:ethereum-testnet-sepolia")
	}
	if onchain.Label != "ethereum-testnet-sepolia (0xaE55...1135)" {
		t.Errorf("on-chain Label = %q, want %q", onchain.Label, "ethereum-testnet-sepolia (0xaE55...1135)")
	}
	if onchain.Type != "on-chain" {
		t.Errorf("on-chain Type = %q, want %q", onchain.Type, "on-chain")
	}
	if onchain.Address == nil || *onchain.Address != "0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135" {
		t.Errorf("on-chain Address unexpected: %v", onchain.Address)
	}
	if len(onchain.SecretsAuthFlows) != 2 || onchain.SecretsAuthFlows[0] != "browser" || onchain.SecretsAuthFlows[1] != "owner-key-signing" {
		t.Errorf("on-chain SecretsAuthFlows = %v, want [browser owner-key-signing]", onchain.SecretsAuthFlows)
	}

	private := envCtx.Registries[1]
	if private.ID != "private" {
		t.Errorf("private ID = %q, want %q", private.ID, "private")
	}
	if private.Type != "off-chain" {
		t.Errorf("private Type = %q, want %q", private.Type, "off-chain")
	}
	if len(private.SecretsAuthFlows) != 1 || private.SecretsAuthFlows[0] != "browser" {
		t.Errorf("private SecretsAuthFlows = %v, want [browser]", private.SecretsAuthFlows)
	}
}

func TestFetchAndWriteContext_PrivateOnly(t *testing.T) {
	srv := newMockGQLServer(t, gqlResponsePrivateOnly())
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	log := testutil.NewTestLogger()
	client := newGQLClient(t, srv.URL)

	err := FetchAndWriteContext(context.Background(), client, "PRODUCTION", log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	envCtx, err := LoadContext("PRODUCTION")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if len(envCtx.Registries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(envCtx.Registries))
	}
	if envCtx.Registries[0].ID != "private" {
		t.Errorf("ID = %q, want %q", envCtx.Registries[0].ID, "private")
	}
}

func TestFetchAndWriteContext_GQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "unauthorized"}},
		})
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	log := testutil.NewTestLogger()
	client := newGQLClient(t, srv.URL)

	err := FetchAndWriteContext(context.Background(), client, "STAGING", log)
	if err == nil {
		t.Fatal("expected error for GQL error response")
	}
}

func TestFetchAndWriteContext_EnvNameUppercased(t *testing.T) {
	srv := newMockGQLServer(t, gqlResponsePrivateOnly())
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	log := testutil.NewTestLogger()
	client := newGQLClient(t, srv.URL)

	if err := FetchAndWriteContext(context.Background(), client, "staging", log); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be loadable with any casing
	if _, err := LoadContext("STAGING"); err != nil {
		t.Errorf("failed to load with uppercase: %v", err)
	}
	if _, err := LoadContext("staging"); err != nil {
		t.Errorf("failed to load with lowercase: %v", err)
	}
}

// --- LoadContextFromPath ---

func TestLoadContextFromPath_Valid(t *testing.T) {
	content := `STAGING:
  tenant_id: "1"
  default_don_family: zone-a
  vault_gateway_url: https://gw.example.com/
  registries:
  - id: private
    label: Private
    type: off-chain
    secrets_auth_flows:
    - browser
`
	path := filepath.Join(t.TempDir(), ContextFile)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	envCtx, err := LoadContextFromPath(path, "STAGING")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if envCtx.TenantID != "1" {
		t.Errorf("TenantID = %q, want %q", envCtx.TenantID, "1")
	}
	if len(envCtx.Registries) != 1 || envCtx.Registries[0].ID != "private" {
		t.Errorf("unexpected registries: %+v", envCtx.Registries)
	}
}

func TestLoadContextFromPath_MissingFile(t *testing.T) {
	_, err := LoadContextFromPath("/nonexistent/path/context.yaml", "STAGING")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadContextFromPath_BadYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), ContextFile)
	if err := os.WriteFile(path, []byte("not: [valid: yaml: {{"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadContextFromPath(path, "STAGING")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadContextFromPath_UnknownEnvironment(t *testing.T) {
	content := `PRODUCTION:
  tenant_id: "1"
  registries: []
`
	path := filepath.Join(t.TempDir(), ContextFile)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadContextFromPath(path, "STAGING")
	if err == nil {
		t.Fatal("expected error for unknown environment")
	}
}

// --- EnsureContext ---

func TestEnsureContext_APIKeyAlwaysFetches(t *testing.T) {
	var callCount atomic.Int32
	srv := newCountingGQLServer(t, &callCount, gqlResponsePrivateOnly())
	defer srv.Close()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	log := testutil.NewTestLogger()
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"}
	envSet := &environments.EnvironmentSet{EnvName: "STAGING", GraphQLURL: srv.URL}

	// First call — no file, should fetch
	if err := EnsureContext(context.Background(), creds, envSet, log); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 GQL call, got %d", callCount.Load())
	}

	// Second call — file exists, API key should still fetch
	if err := EnsureContext(context.Background(), creds, envSet, log); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if callCount.Load() != 2 {
		t.Fatalf("expected 2 GQL calls (API key always fetches), got %d", callCount.Load())
	}
}

func TestEnsureContext_BearerUsesCached(t *testing.T) {
	var callCount atomic.Int32
	srv := newCountingGQLServer(t, &callCount, gqlResponsePrivateOnly())
	defer srv.Close()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	log := testutil.NewTestLogger()
	creds := &credentials.Credentials{
		AuthType: credentials.AuthTypeBearer,
		Tokens:   &credentials.CreLoginTokenSet{AccessToken: fakeJWT(t)},
	}
	envSet := &environments.EnvironmentSet{EnvName: "STAGING", GraphQLURL: srv.URL}

	// First call — no file, should fetch
	if err := EnsureContext(context.Background(), creds, envSet, log); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 GQL call, got %d", callCount.Load())
	}

	// Second call — file exists, bearer should use cache
	if err := EnsureContext(context.Background(), creds, envSet, log); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 GQL call (bearer uses cache), got %d", callCount.Load())
	}
}

func TestEnsureContext_DefaultsToProduction(t *testing.T) {
	srv := newMockGQLServer(t, gqlResponsePrivateOnly())
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	log := testutil.NewTestLogger()
	creds := &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"}
	envSet := &environments.EnvironmentSet{EnvName: "", GraphQLURL: srv.URL}

	if err := EnsureContext(context.Background(), creds, envSet, log); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be stored under PRODUCTION
	if _, err := LoadContext("PRODUCTION"); err != nil {
		t.Errorf("expected PRODUCTION block: %v", err)
	}
}

// --- Helper functions ---

func TestMapRegistryType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ON_CHAIN", "on-chain"},
		{"OFF_CHAIN", "off-chain"},
		{"UNKNOWN", "unknown"},
	}
	for _, tt := range tests {
		if got := mapRegistryType(tt.input); got != tt.want {
			t.Errorf("mapRegistryType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMapSecretsAuthFlows(t *testing.T) {
	got := mapSecretsAuthFlows([]string{"BROWSER", "OWNER_KEY_SIGNING", "FUTURE_FLOW"})
	want := []string{"browser", "owner-key-signing", "future_flow"}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAbbreviateAddress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0xaE55eB3EDAc48a1163EE2cbb1205bE1e90Ea1135", "0xaE55...1135"},
		{"0x12345678", "0x12345678"}, // 10 chars, no abbreviation
		{"short", "short"},
	}
	for _, tt := range tests {
		if got := abbreviateAddress(tt.input); got != tt.want {
			t.Errorf("abbreviateAddress(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

package tenderly

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func newTestProvider(t *testing.T, serverURL string) *APIProvider {
	t.Helper()
	return &APIProvider{
		accessKey:   "test-key",
		accountSlug: "test-account",
		projectSlug: "test-project",
		userID:      "auth0|user123",
		httpClient:  &http.Client{},
		baseURL:     serverURL,
	}
}

func TestAPIProviderCreateVnets(t *testing.T) {
	t.Run("creates vnet and returns RPC URL from Admin RPC", func(t *testing.T) {
		vnetCount := 0
		server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "test-key", r.Header.Get("X-Access-Key"))

			var body createVnetRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, 11155111, body.ForkConfig.NetworkID)
			require.Contains(t, body.Slug, "auth0-user123", "slug should contain sanitized user ID")

			vnetCount++
			resp := createVnetResponse{
				ID: "vnet-abc123",
				RPCs: []rpcEntry{
					{Name: "Admin RPC", URL: "https://virtual.mainnet.rpc.tenderly.co/abc123"},
					{Name: "Public RPC", URL: "https://virtual.mainnet.rpc.tenderly.co/pub-abc123"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		})
		defer server.Close()

		p := newTestProvider(t, server.URL)
		result, err := p.CreateVnets([]string{"ethereum-testnet-sepolia"})

		require.NoError(t, err)
		require.Equal(t, "https://virtual.mainnet.rpc.tenderly.co/abc123", result.NetworkRPCs["ethereum-testnet-sepolia"])
		require.Contains(t, result.VnetURLs["ethereum-testnet-sepolia"], "vnet-abc123")
		require.Equal(t, 1, vnetCount)
	})

	t.Run("creates one vnet per network", func(t *testing.T) {
		var requestedNetIDs []int
		server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			var body createVnetRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			requestedNetIDs = append(requestedNetIDs, body.ForkConfig.NetworkID)

			resp := createVnetResponse{
				ID:   "vnet-multi",
				RPCs: []rpcEntry{{Name: "Admin RPC", URL: "https://rpc.tenderly.co/" + body.Slug}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		})
		defer server.Close()

		p := newTestProvider(t, server.URL)
		networks := []string{"ethereum-testnet-sepolia", "base-sepolia", "arbitrum-mainnet"}
		result, err := p.CreateVnets(networks)

		require.NoError(t, err)
		require.Len(t, result.NetworkRPCs, 3)
		require.ElementsMatch(t, []int{11155111, 84532, 42161}, requestedNetIDs)
	})

	t.Run("returns error for unsupported network", func(t *testing.T) {
		server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("should not reach the API for unsupported network")
		})
		defer server.Close()

		p := newTestProvider(t, server.URL)
		_, err := p.CreateVnets([]string{"unknown-chain"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported network")
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid key"}`))
		})
		defer server.Close()

		p := newTestProvider(t, server.URL)
		_, err := p.CreateVnets([]string{"ethereum-testnet-sepolia"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "401")
	})

	t.Run("falls back to first RPC when no Admin RPC", func(t *testing.T) {
		server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			resp := createVnetResponse{
				ID:   "vnet-fallback",
				RPCs: []rpcEntry{{Name: "Public RPC", URL: "https://rpc.tenderly.co/pub123"}},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		})
		defer server.Close()

		p := newTestProvider(t, server.URL)
		result, err := p.CreateVnets([]string{"ethereum-mainnet"})

		require.NoError(t, err)
		require.Equal(t, "https://rpc.tenderly.co/pub123", result.NetworkRPCs["ethereum-mainnet"])
	})
}

func TestNewAPIProviderEnvVars(t *testing.T) {
	t.Run("returns error when access key is missing", func(t *testing.T) {
		t.Setenv(EnvAccessKey, "")
		_, err := NewAPIProvider("auth0|user1")
		require.Error(t, err)
		require.Contains(t, err.Error(), EnvAccessKey)
	})

	t.Run("returns error when account slug is missing", func(t *testing.T) {
		t.Setenv(EnvAccessKey, "key")
		t.Setenv(EnvAccountSlug, "")
		_, err := NewAPIProvider("auth0|user1")
		require.Error(t, err)
		require.Contains(t, err.Error(), EnvAccountSlug)
	})

	t.Run("returns error when project slug is missing", func(t *testing.T) {
		t.Setenv(EnvAccessKey, "key")
		t.Setenv(EnvAccountSlug, "acct")
		t.Setenv(EnvProjectSlug, "")
		_, err := NewAPIProvider("auth0|user1")
		require.Error(t, err)
		require.Contains(t, err.Error(), EnvProjectSlug)
	})

	t.Run("returns error when user ID is empty", func(t *testing.T) {
		t.Setenv(EnvAccessKey, "key")
		t.Setenv(EnvAccountSlug, "acct")
		t.Setenv(EnvProjectSlug, "proj")
		_, err := NewAPIProvider("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "user ID")
	})

	t.Run("succeeds when all env vars and user ID are set", func(t *testing.T) {
		t.Setenv(EnvAccessKey, "key")
		t.Setenv(EnvAccountSlug, "acct")
		t.Setenv(EnvProjectSlug, "proj")
		p, err := NewAPIProvider("auth0|user1")
		require.NoError(t, err)
		require.Equal(t, "key", p.accessKey)
		require.Equal(t, "acct", p.accountSlug)
		require.Equal(t, "proj", p.projectSlug)
		require.Equal(t, "auth0|user1", p.userID)
	})
}

func TestNetworkIDMapping(t *testing.T) {
	tests := []struct {
		network    string
		expectedID int
		supported  bool
	}{
		{"ethereum-mainnet", 1, true},
		{"ethereum-testnet-sepolia", 11155111, true},
		{"base-sepolia", 84532, true},
		{"arbitrum-mainnet", 42161, true},
		{"unknown-chain", 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.network, func(t *testing.T) {
			id, ok := NetworkID(tc.network)
			require.Equal(t, tc.supported, ok)
			require.Equal(t, tc.expectedID, id)
		})
	}
}

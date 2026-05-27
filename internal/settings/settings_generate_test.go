package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
)

func TestBuildRPCsListYAML(t *testing.T) {
	t.Run("with networks and URLs", func(t *testing.T) {
		yaml := BuildRPCsListYAML(
			[]string{"ethereum-testnet-sepolia", "ethereum-mainnet"},
			map[string]string{
				"ethereum-testnet-sepolia": "https://sepolia.example.com",
				"ethereum-mainnet":         "https://mainnet.example.com",
			},
		)
		assert.Contains(t, yaml, "chain-name: ethereum-testnet-sepolia")
		assert.Contains(t, yaml, "url: https://sepolia.example.com")
		assert.Contains(t, yaml, "chain-name: ethereum-mainnet")
		assert.Contains(t, yaml, "url: https://mainnet.example.com")
	})

	t.Run("with partial URLs leaves blank", func(t *testing.T) {
		yaml := BuildRPCsListYAML(
			[]string{"ethereum-testnet-sepolia", "base-sepolia"},
			map[string]string{
				"ethereum-testnet-sepolia": "https://sepolia.example.com",
			},
		)
		assert.Contains(t, yaml, "chain-name: ethereum-testnet-sepolia")
		assert.Contains(t, yaml, "url: https://sepolia.example.com")
		assert.Contains(t, yaml, "chain-name: base-sepolia")
		// base-sepolia has no URL provided, should be blank
		assert.Contains(t, yaml, "url: \n")
	})

	t.Run("empty networks falls back to default", func(t *testing.T) {
		yaml := BuildRPCsListYAML(nil, nil)
		assert.Contains(t, yaml, "chain-name: "+constants.DefaultEthSepoliaChainName)
		assert.Contains(t, yaml, "url: "+constants.DefaultEthSepoliaRpcUrl)
	})

	t.Run("proper YAML indentation", func(t *testing.T) {
		yaml := BuildRPCsListYAML(
			[]string{"ethereum-testnet-sepolia"},
			map[string]string{"ethereum-testnet-sepolia": "https://rpc.example.com"},
		)
		require.Contains(t, yaml, "  rpcs:\n")
		require.Contains(t, yaml, "    - chain-name: ")
		require.Contains(t, yaml, "      url: ")
	})
}

func TestGetReplacementsWithNetworks(t *testing.T) {
	repl := GetReplacementsWithNetworks(
		[]string{"ethereum-testnet-sepolia"},
		map[string]string{"ethereum-testnet-sepolia": "https://rpc.example.com"},
	)
	assert.Contains(t, repl, "RPCsList")
	assert.Contains(t, repl["RPCsList"], "chain-name: ethereum-testnet-sepolia")
	// Should still have all default replacements
	assert.Contains(t, repl, "ConfigPathStaging")
}

func TestPatchProjectRPCs(t *testing.T) {
	t.Run("patches matching chain URLs", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, "project.yaml")

		original := `# comment preserved
staging-settings:
  rpcs:
    - chain-name: ethereum-testnet-sepolia
      url: https://old-sepolia.com
    - chain-name: ethereum-mainnet
      url: https://old-mainnet.com
production-settings:
  rpcs:
    - chain-name: ethereum-testnet-sepolia
      url: https://old-sepolia.com
    - chain-name: ethereum-mainnet
      url: https://old-mainnet.com
`
		require.NoError(t, os.WriteFile(yamlPath, []byte(original), 0600))

		err := PatchProjectRPCs(yamlPath, map[string]string{
			"ethereum-testnet-sepolia": "https://new-sepolia.com",
		})
		require.NoError(t, err)

		content, err := os.ReadFile(yamlPath)
		require.NoError(t, err)
		s := string(content)

		// Patched chain should have new URL
		assert.Contains(t, s, "https://new-sepolia.com")
		// Unmatched chain should keep original URL
		assert.Contains(t, s, "https://old-mainnet.com")
		// Old URL should be gone for patched chain
		assert.NotContains(t, s, "https://old-sepolia.com")
		// Both sections should be patched
		assert.Contains(t, s, "staging-settings")
		assert.Contains(t, s, "production-settings")
	})

	t.Run("no-op with empty rpcURLs", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, "project.yaml")

		original := `staging-settings:
  rpcs:
    - chain-name: ethereum-testnet-sepolia
      url: https://original.com
`
		require.NoError(t, os.WriteFile(yamlPath, []byte(original), 0600))

		err := PatchProjectRPCs(yamlPath, map[string]string{})
		require.NoError(t, err)

		content, err := os.ReadFile(yamlPath)
		require.NoError(t, err)
		// File should be unchanged
		assert.Equal(t, original, string(content))
	})

	t.Run("skips empty URL values", func(t *testing.T) {
		tmpDir := t.TempDir()
		yamlPath := filepath.Join(tmpDir, "project.yaml")

		original := `staging-settings:
  rpcs:
    - chain-name: ethereum-testnet-sepolia
      url: https://original.com
`
		require.NoError(t, os.WriteFile(yamlPath, []byte(original), 0600))

		err := PatchProjectRPCs(yamlPath, map[string]string{
			"ethereum-testnet-sepolia": "",
		})
		require.NoError(t, err)

		content, err := os.ReadFile(yamlPath)
		require.NoError(t, err)
		// Original URL should be preserved when user provides empty value
		assert.Contains(t, string(content), "https://original.com")
	})
}

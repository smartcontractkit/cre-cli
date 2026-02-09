package settings

import (
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
				"ethereum-mainnet":        "https://mainnet.example.com",
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

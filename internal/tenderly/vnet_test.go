package tenderly

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvProviderCreateVnets(t *testing.T) {
	t.Run("returns correct map when env var is set", func(t *testing.T) {
		t.Setenv(EnvVnetURL, "https://rpc.tenderly.co/vnet/abc123")

		p := NewEnvProvider()
		result, err := p.CreateVnets([]string{"ethereum-testnet-sepolia"})

		require.NoError(t, err)
		require.Equal(t, "https://rpc.tenderly.co/vnet/abc123", result.NetworkRPCs["ethereum-testnet-sepolia"])
		require.Equal(t, "https://rpc.tenderly.co/vnet/abc123", result.VnetURLs["ethereum-testnet-sepolia"])
	})

	t.Run("returns error with guidance when env var is unset", func(t *testing.T) {
		t.Setenv(EnvVnetURL, "")

		p := NewEnvProvider()
		result, err := p.CreateVnets([]string{"ethereum-testnet-sepolia"})

		require.Nil(t, result)
		require.Error(t, err)
		require.Contains(t, err.Error(), EnvVnetURL)
		require.Contains(t, err.Error(), "export")
	})

	t.Run("multiple networks all get the same base URL", func(t *testing.T) {
		t.Setenv(EnvVnetURL, "https://rpc.tenderly.co/vnet/multi")

		p := NewEnvProvider()
		networks := []string{"ethereum-testnet-sepolia", "ethereum-mainnet", "arbitrum-mainnet"}
		result, err := p.CreateVnets(networks)

		require.NoError(t, err)
		require.Len(t, result.NetworkRPCs, 3)
		for _, network := range networks {
			require.Equal(t, "https://rpc.tenderly.co/vnet/multi", result.NetworkRPCs[network])
			require.Equal(t, "https://rpc.tenderly.co/vnet/multi", result.VnetURLs[network])
		}
	})
}

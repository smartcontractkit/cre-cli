package runtime

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestValidateOnchainRegistryRPC(t *testing.T) {
	t.Parallel()

	const chainName = "ethereum-testnet-sepolia"

	// A Context wired with one RPC entry for the registry chain.
	ctxWithRPC := func(registry settings.ResolvedRegistry) *Context {
		return &Context{
			ResolvedRegistry: registry,
			EnvironmentSet: &environments.EnvironmentSet{
				WorkflowRegistryChainName: chainName,
			},
			Settings: &settings.Settings{
				Workflow: settings.WorkflowSettings{
					RPCs: []settings.RpcEndpoint{
						{ChainName: chainName, Url: "https://rpc.example.com"},
					},
				},
			},
		}
	}

	// A Context with no RPC entries at all.
	ctxWithoutRPC := func(registry settings.ResolvedRegistry) *Context {
		return &Context{
			ResolvedRegistry: registry,
			EnvironmentSet: &environments.EnvironmentSet{
				WorkflowRegistryChainName: chainName,
			},
			Settings: &settings.Settings{
				Workflow: settings.WorkflowSettings{},
			},
		}
	}

	t.Run("off-chain registry: always a no-op", func(t *testing.T) {
		t.Parallel()
		// Even without any RPC URLs configured, off-chain registry must not error.
		offChain := settings.NewOffChainRegistry("private", "zone-a")
		ctx := ctxWithoutRPC(offChain)
		require.NoError(t, ctx.ValidateOnchainRegistryRPC())
	})

	t.Run("on-chain registry with valid RPC: passes", func(t *testing.T) {
		t.Parallel()
		onChain := settings.NewOnChainRegistry("onchain:"+chainName, "0xabc", chainName, "zone-a", "")
		ctx := ctxWithRPC(onChain)
		require.NoError(t, ctx.ValidateOnchainRegistryRPC())
	})

	t.Run("on-chain registry without RPC: returns error", func(t *testing.T) {
		t.Parallel()
		onChain := settings.NewOnChainRegistry("onchain:"+chainName, "0xabc", chainName, "zone-a", "")
		ctx := ctxWithoutRPC(onChain)
		err := ctx.ValidateOnchainRegistryRPC()
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing RPC URL")
	})

	t.Run("nil resolved registry (default on-chain path) with valid RPC: passes", func(t *testing.T) {
		t.Parallel()
		ctx := ctxWithRPC(nil)
		require.NoError(t, ctx.ValidateOnchainRegistryRPC())
	})

	t.Run("nil resolved registry (default on-chain path) without RPC: returns error", func(t *testing.T) {
		t.Parallel()
		ctx := ctxWithoutRPC(nil)
		err := ctx.ValidateOnchainRegistryRPC()
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing RPC URL")
	})
}

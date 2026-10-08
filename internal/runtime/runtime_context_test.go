package runtime

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/creconfig"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
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

func TestTryAttachCredentials(t *testing.T) {
	discardLogger := zerolog.New(io.Discard)

	t.Run("attaches API key credentials from environment", func(t *testing.T) {
		t.Setenv(credentials.CreApiKeyVar, "test-api-key")

		ctx := &Context{Logger: &discardLogger}
		ctx.TryAttachCredentials()

		require.NotNil(t, ctx.Credentials)
		require.Equal(t, "test-api-key", ctx.Credentials.APIKey)
		require.Equal(t, credentials.AuthTypeApiKey, ctx.Credentials.AuthType)
	})

	t.Run("attaches bearer credentials from config file", func(t *testing.T) {
		t.Setenv(credentials.CreApiKeyVar, "")
		home := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(home, creconfig.Dir), 0o700))
		require.NoError(t, os.WriteFile(
			filepath.Join(home, creconfig.Dir, credentials.ConfigFile),
			[]byte("AccessToken: test-access-token\n"),
			0o600,
		))
		t.Setenv("HOME", home)

		ctx := &Context{Logger: &discardLogger}
		ctx.TryAttachCredentials()

		require.NotNil(t, ctx.Credentials)
		require.NotNil(t, ctx.Credentials.Tokens)
		require.Equal(t, "test-access-token", ctx.Credentials.Tokens.AccessToken)
	})

	t.Run("leaves credentials nil when none exist", func(t *testing.T) {
		t.Setenv(credentials.CreApiKeyVar, "")
		t.Setenv("HOME", t.TempDir())

		ctx := &Context{Logger: &discardLogger}
		ctx.TryAttachCredentials()

		require.Nil(t, ctx.Credentials)
	})
}

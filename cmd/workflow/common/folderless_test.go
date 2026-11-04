package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/testutil/chainsim"
)

func TestIsFolderlessInvocation(t *testing.T) {
	t.Parallel()

	existingDir := t.TempDir()

	// A project layout where the workflow dir only exists relative to the
	// project root, not the invocation directory (the legacy resolution mode).
	projectRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "project.yaml"), []byte("staging-settings: {}"), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(projectRoot, "blank_workflow"), 0755))

	tests := []struct {
		name            string
		args            []string
		projectRootFlag string
		want            bool
	}{
		{"no args (picker mode)", nil, "", true},
		{"workflow name", []string{"my-workflow"}, "", true},
		{"on-chain workflow ID", []string{"00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856"}, "", true},
		{"existing directory", []string{existingDir}, "", false},
		{"current directory", []string{"."}, "", false},
		{"parent directory", []string{".."}, "", false},
		{"relative path", []string{"./my-workflow"}, "", false},
		{"nested relative path", []string{"workflows/my-workflow"}, "", false},
		{"absolute path", []string{filepath.Join(string(os.PathSeparator), "tmp", "my-workflow")}, "", false},
		{"name that is a directory under --project-root", []string{"blank_workflow"}, projectRoot, false},
		{"unknown name with --project-root", []string{"other-workflow"}, projectRoot, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsFolderlessInvocation(tt.args, tt.projectRootFlag))
		})
	}
}

func folderlessRuntimeContext(registries []*tenantctx.Registry) *runtime.Context {
	return &runtime.Context{
		EnvironmentSet: &environments.EnvironmentSet{
			WorkflowRegistryChainExplorerURL: "https://explorer.example",
		},
		TenantContext: &tenantctx.EnvironmentContext{
			DefaultDonFamily: "don-a",
			Registries:       registries,
		},
	}
}

func TestRegistryFromWorkflowSource(t *testing.T) {
	t.Parallel()

	onChainReg := &tenantctx.Registry{
		ID:            "onchain-sepolia",
		Type:          "on-chain",
		Address:       strPtr("0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D"),
		ChainSelector: strPtr("16015286601757825753"),
	}
	privateReg := &tenantctx.Registry{ID: "private", Type: "off-chain"}
	sepoliaSource := "contract:16015286601757825753:0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D"

	t.Run("private source matched in tenant context", func(t *testing.T) {
		t.Parallel()
		reg, err := RegistryFromWorkflowSource(folderlessRuntimeContext([]*tenantctx.Registry{privateReg, onChainReg}), "private")
		require.NoError(t, err)
		assert.Equal(t, settings.RegistryTypeOffChain, reg.Type())
		assert.Equal(t, "private", reg.ID())
		assert.Equal(t, "don-a", reg.DonFamily())
	})

	t.Run("contract source matched in tenant context", func(t *testing.T) {
		t.Parallel()
		reg, err := RegistryFromWorkflowSource(folderlessRuntimeContext([]*tenantctx.Registry{privateReg, onChainReg}), sepoliaSource)
		require.NoError(t, err)
		assert.Equal(t, settings.RegistryTypeOnChain, reg.Type())
		assert.Equal(t, "onchain-sepolia", reg.ID())

		oc, ok := reg.(*settings.OnChainRegistry)
		require.True(t, ok)
		assert.Equal(t, "0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D", oc.Address())
		assert.Equal(t, "ethereum-testnet-sepolia", oc.ChainName())
		assert.Equal(t, "https://explorer.example", oc.ExplorerURL())
	})

	t.Run("contract source fallback without tenant match", func(t *testing.T) {
		t.Parallel()
		reg, err := RegistryFromWorkflowSource(folderlessRuntimeContext(nil), sepoliaSource)
		require.NoError(t, err)
		assert.Equal(t, settings.RegistryTypeOnChain, reg.Type())
		assert.Equal(t, sepoliaSource, reg.ID())

		oc, ok := reg.(*settings.OnChainRegistry)
		require.True(t, ok)
		assert.Equal(t, "0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D", oc.Address())
		assert.Equal(t, "ethereum-testnet-sepolia", oc.ChainName())
	})

	t.Run("contract source with unknown chain selector", func(t *testing.T) {
		t.Parallel()
		_, err := RegistryFromWorkflowSource(folderlessRuntimeContext(nil), "contract:99999999999999999999:0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot resolve registry chain")
	})

	t.Run("grpc source falls back to off-chain", func(t *testing.T) {
		t.Parallel()
		reg, err := RegistryFromWorkflowSource(folderlessRuntimeContext(nil), "grpc:some-registry")
		require.NoError(t, err)
		assert.Equal(t, settings.RegistryTypeOffChain, reg.Type())
		assert.Equal(t, "grpc:some-registry", reg.ID())
	})

	t.Run("empty source defaults to private", func(t *testing.T) {
		t.Parallel()
		reg, err := RegistryFromWorkflowSource(folderlessRuntimeContext(nil), "")
		require.NoError(t, err)
		assert.Equal(t, settings.RegistryTypeOffChain, reg.Type())
		assert.Equal(t, "private", reg.ID())
	})

	t.Run("unsupported registry type errors", func(t *testing.T) {
		t.Parallel()
		future := &tenantctx.Registry{ID: "future", Type: "FUTURE_TYPE"}
		_, err := RegistryFromWorkflowSource(folderlessRuntimeContext([]*tenantctx.Registry{future}), "future")
		require.Error(t, err)
	})
}

func TestVerifyEOAOwner(t *testing.T) {
	t.Parallel()

	t.Run("matching key passes", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		v.Set(settings.EthPrivateKeyEnvVar, chainsim.TestPrivateKey)
		require.NoError(t, VerifyEOAOwner(v, chainsim.TestAddress))
	})

	t.Run("owner mismatch fails", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		v.Set(settings.EthPrivateKeyEnvVar, chainsim.TestPrivateKey)
		err := VerifyEOAOwner(v, "0x4c0883a69102937d6234146e38a6aefbf95944c6")
		require.Error(t, err)
		assert.Contains(t, err.Error(), settings.EthPrivateKeyEnvVar)
	})

	t.Run("missing key fails", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		err := VerifyEOAOwner(v, chainsim.TestAddress)
		require.Error(t, err)
		assert.Contains(t, err.Error(), settings.EthPrivateKeyEnvVar+" is not set")
	})
}

func TestGuardFolderlessTxnType(t *testing.T) {
	t.Parallel()

	t.Run("unsigned is rejected", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		v.Set(settings.Flags.RawTxFlag.Name, true)
		err := GuardFolderlessTxnType(v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "require a project folder")
	})

	t.Run("changeset is rejected", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		v.Set(settings.Flags.Changeset.Name, true)
		err := GuardFolderlessTxnType(v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "require a project folder")
	})

	t.Run("regular transactions pass", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, GuardFolderlessTxnType(viper.New()))
	})
}

func strPtr(s string) *string { return &s }

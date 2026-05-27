package create

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestNonInteractive_WithoutYes_ReturnsError(t *testing.T) {
	t.Parallel()
	v := viper.New()
	v.Set(settings.Flags.NonInteractive.Name, true)
	v.Set(settings.Flags.SkipConfirmation.Name, false)

	ctx := &runtime.Context{Viper: v}
	cmd := New(ctx)

	err := cmd.RunE(cmd, []string{"/tmp/fake-secrets.yaml"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required flags for --non-interactive mode")
}

func TestNonInteractive_WithYes_PassesGuard(t *testing.T) {
	t.Parallel()
	v := viper.New()
	v.Set(settings.Flags.NonInteractive.Name, true)
	v.Set(settings.Flags.SkipConfirmation.Name, true)

	ctx := &runtime.Context{Viper: v}
	cmd := New(ctx)

	err := cmd.RunE(cmd, []string{"/tmp/fake-secrets.yaml"})
	// Guard passes; error comes from missing runtime setup, not the guard
	if err != nil {
		require.NotContains(t, err.Error(), "missing required flags for --non-interactive mode")
	}
}

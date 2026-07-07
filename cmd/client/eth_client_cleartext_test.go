package client_test

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func TestNewEthClientFromEnvBlocksRemoteCleartext(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set(settings.Flags.AllowInsecureRPC.Name, false)

	logger := zerolog.Nop()
	_, err := client.NewEthClientFromEnv(v, &logger, "http://rpc.example.com")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--allow-insecure-rpc")
}

func TestNewEthClientFromEnvAllowsCleartextWithOptIn(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set(settings.Flags.AllowInsecureRPC.Name, true)

	logger := zerolog.Nop()
	_, err := client.NewEthClientFromEnv(v, &logger, "http://rpc.example.com")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "--allow-insecure-rpc")
}

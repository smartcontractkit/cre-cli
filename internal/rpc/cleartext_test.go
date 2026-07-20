package rpc_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/rpc"
)

func TestEvaluateCleartextRPC(t *testing.T) {
	t.Run("allows https", func(t *testing.T) {
		warn, err := rpc.EvaluateCleartextRPC("https://rpc.example.com/v3/key", rpc.CleartextPolicyOptions{})
		require.NoError(t, err)
		require.Empty(t, warn)
	})

	t.Run("allows loopback http", func(t *testing.T) {
		warn, err := rpc.EvaluateCleartextRPC("http://127.0.0.1:8545", rpc.CleartextPolicyOptions{})
		require.NoError(t, err)
		require.Empty(t, warn)
	})

	t.Run("blocks remote http without opt-in", func(t *testing.T) {
		_, err := rpc.EvaluateCleartextRPC("http://rpc.example.com", rpc.CleartextPolicyOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "--allow-insecure-rpc")
	})

	t.Run("blocks remote http with path key without opt-in", func(t *testing.T) {
		_, err := rpc.EvaluateCleartextRPC("http://rpc.example.com/v3/secret-key", rpc.CleartextPolicyOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "--allow-insecure-rpc")
	})

	t.Run("allows remote http with opt-in", func(t *testing.T) {
		warn, err := rpc.EvaluateCleartextRPC("http://rpc.example.com/v3/secret-key", rpc.CleartextPolicyOptions{
			AllowInsecure: true,
		})
		require.NoError(t, err)
		require.Contains(t, warn, "insecure")
	})

	t.Run("rejects invalid URL", func(t *testing.T) {
		_, err := rpc.EvaluateCleartextRPC("ftp://rpc.example.com", rpc.CleartextPolicyOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid scheme")
	})
}

func TestIsLoopbackHost(t *testing.T) {
	require.True(t, rpc.IsLoopbackHost("localhost"))
	require.True(t, rpc.IsLoopbackHost("127.0.0.1"))
	require.True(t, rpc.IsLoopbackHost("::1"))
	require.False(t, rpc.IsLoopbackHost("rpc.example.com"))
	require.False(t, rpc.IsLoopbackHost("10.0.0.1"))
}

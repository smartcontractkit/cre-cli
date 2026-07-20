package rpc_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/rpc"
)

func TestIsValidURL(t *testing.T) {
	t.Run("accepts https URL", func(t *testing.T) {
		require.NoError(t, rpc.IsValidURL("https://rpc.example.com"))
	})

	t.Run("accepts http URL", func(t *testing.T) {
		require.NoError(t, rpc.IsValidURL("http://127.0.0.1:8545"))
	})

	t.Run("rejects invalid scheme", func(t *testing.T) {
		err := rpc.IsValidURL("ftp://rpc.example.com")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid scheme")
	})

	t.Run("rejects missing host", func(t *testing.T) {
		err := rpc.IsValidURL("https://")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid host")
	})

	t.Run("rejects URL without scheme", func(t *testing.T) {
		err := rpc.IsValidURL("not-a-valid-url")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid scheme")
	})
}

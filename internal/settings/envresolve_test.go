package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveEnvVars(t *testing.T) {
	t.Run("plain URL without vars returned unchanged", func(t *testing.T) {
		t.Parallel()
		result, err := ResolveEnvVars("https://rpc.example.com/v1/abc123")
		require.NoError(t, err)
		assert.Equal(t, "https://rpc.example.com/v1/abc123", result)
	})

	t.Run("single var at end of URL resolves", func(t *testing.T) {
		t.Setenv("TEST_RPC_KEY", "my-secret-key")
		result, err := ResolveEnvVars("https://rpc.example.com/${TEST_RPC_KEY}")
		require.NoError(t, err)
		assert.Equal(t, "https://rpc.example.com/my-secret-key", result)
	})

	t.Run("multiple vars resolve", func(t *testing.T) {
		t.Setenv("TEST_HOST", "rpc.example.com")
		t.Setenv("TEST_KEY", "abc123")
		result, err := ResolveEnvVars("https://${TEST_HOST}/v1/${TEST_KEY}")
		require.NoError(t, err)
		assert.Equal(t, "https://rpc.example.com/v1/abc123", result)
	})

	t.Run("var in middle of URL resolves", func(t *testing.T) {
		t.Setenv("TEST_MID_VAR", "segment")
		result, err := ResolveEnvVars("https://rpc.example.com/${TEST_MID_VAR}/endpoint")
		require.NoError(t, err)
		assert.Equal(t, "https://rpc.example.com/segment/endpoint", result)
	})

	t.Run("missing env var returns error", func(t *testing.T) {
		_, err := ResolveEnvVars("https://rpc.example.com/${ENVRESOLVE_TEST_MISSING_VAR}")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `environment variable "ENVRESOLVE_TEST_MISSING_VAR"`)
		assert.Contains(t, err.Error(), "not set")
	})

	t.Run("empty env var value resolves to empty", func(t *testing.T) {
		t.Setenv("TEST_EMPTY_VAR", "")
		result, err := ResolveEnvVars("https://rpc.example.com/${TEST_EMPTY_VAR}")
		require.NoError(t, err)
		assert.Equal(t, "https://rpc.example.com/", result)
	})

	t.Run("dollar var without braces is not resolved", func(t *testing.T) {
		t.Setenv("TEST_NO_BRACES", "value")
		result, err := ResolveEnvVars("https://rpc.example.com/$TEST_NO_BRACES")
		require.NoError(t, err)
		assert.Equal(t, "https://rpc.example.com/$TEST_NO_BRACES", result)
	})
}

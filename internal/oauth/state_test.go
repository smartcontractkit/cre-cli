package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomState(t *testing.T) {
	s, err := RandomState()
	require.NoError(t, err)
	require.NotEmpty(t, s)
	s2, err := RandomState()
	require.NoError(t, err)
	assert.NotEqual(t, s, s2)
}

func TestAuthorizeURLWithState(t *testing.T) {
	t.Run("adds state to URL without existing state", func(t *testing.T) {
		out, err := AuthorizeURLWithState("https://id.example/authorize?client_id=x&response_type=code", "local-state")
		require.NoError(t, err)
		assert.Contains(t, out, "state=local-state")
		assert.Contains(t, out, "client_id=x")
	})

	t.Run("replaces existing state", func(t *testing.T) {
		out, err := AuthorizeURLWithState("https://id.example/authorize?state=platform&client_id=x", "local-state")
		require.NoError(t, err)
		assert.Contains(t, out, "state=local-state")
		assert.NotContains(t, out, "state=platform")
	})

	t.Run("rejects empty state", func(t *testing.T) {
		_, err := AuthorizeURLWithState("https://id.example/authorize", "")
		assert.Error(t, err)
	})

	t.Run("rejects invalid URL", func(t *testing.T) {
		_, err := AuthorizeURLWithState("://bad", "local-state")
		assert.Error(t, err)
	})
}

func TestStateFromAuthorizeURL(t *testing.T) {
	s, err := StateFromAuthorizeURL("https://id.example/authorize?state=abc&client_id=x")
	require.NoError(t, err)
	assert.Equal(t, "abc", s)

	s, err = StateFromAuthorizeURL("https://id.example/authorize")
	require.NoError(t, err)
	assert.Equal(t, "", s)
}

func TestClientIDFromAuthorizeURL(t *testing.T) {
	c, err := ClientIDFromAuthorizeURL("https://auth0.example/authorize?client_id=myapp&response_type=code")
	require.NoError(t, err)
	assert.Equal(t, "myapp", c)
}

func TestOAuthServerBaseFromAuthorizeURL(t *testing.T) {
	base, err := OAuthServerBaseFromAuthorizeURL("https://tenant.auth0.com/authorize?foo=1")
	require.NoError(t, err)
	assert.Equal(t, "https://tenant.auth0.com", base)

	_, err = OAuthServerBaseFromAuthorizeURL("/relative")
	assert.Error(t, err)
}

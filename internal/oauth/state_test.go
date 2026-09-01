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

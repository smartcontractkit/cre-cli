package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

func TestExchangeAuthorizationCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "cid", r.Form.Get("client_id"))
		assert.Equal(t, "auth-code", r.Form.Get("code"))
		assert.Equal(t, constants.AuthRedirectURI, r.Form.Get("redirect_uri"))
		assert.Equal(t, "verifier", r.Form.Get("code_verifier"))

		_ = json.NewEncoder(w).Encode(credentials.CreLoginTokenSet{
			AccessToken: "a",
			TokenType:   "Bearer",
		})
	}))
	defer ts.Close()

	env := &environments.EnvironmentSet{
		AuthBase: ts.URL,
		ClientID: "cid",
	}

	tok, err := ExchangeAuthorizationCode(context.Background(), ts.Client(), env, "auth-code", "verifier")
	require.NoError(t, err)
	require.NotNil(t, tok)
	assert.Equal(t, "a", tok.AccessToken)
}

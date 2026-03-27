package oauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestSecretsCallbackHandler_success(t *testing.T) {
	log := zerolog.Nop()
	codeCh := make(chan string, 1)
	h := SecretsCallbackHandler(codeCh, "want-state", &log)

	req := httptest.NewRequest(http.MethodGet, "/callback?code=the-code&state=want-state", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "the-code", <-codeCh)
}

func TestSecretsCallbackHandler_stateMismatch(t *testing.T) {
	log := zerolog.Nop()
	codeCh := make(chan string, 1)
	h := SecretsCallbackHandler(codeCh, "want", &log)

	req := httptest.NewRequest(http.MethodGet, "/callback?code=c&state=wrong", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	select {
	case <-codeCh:
		t.Fatal("expected no code")
	default:
	}
}

func TestSecretsCallbackHandler_oauthError(t *testing.T) {
	log := zerolog.Nop()
	codeCh := make(chan string, 1)
	h := SecretsCallbackHandler(codeCh, "", &log)

	req := httptest.NewRequest(http.MethodGet, "/callback?error=access_denied", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Len(t, codeCh, 0)
}

func TestSecretsCallbackHandler_noStateRequired(t *testing.T) {
	log := zerolog.Nop()
	codeCh := make(chan string, 1)
	h := SecretsCallbackHandler(codeCh, "", &log)

	req := httptest.NewRequest(http.MethodGet, "/callback?code=only-code", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "only-code", <-codeCh)
}

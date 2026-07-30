package oauth

import (
	"net/http"

	"github.com/rs/zerolog"
)

// SecretsCallbackHandler handles the OAuth redirect for the browser secrets flow.
// If expectedState is non-empty (parsed from the platform authorize URL), the callback
// must include the same state; otherwise only a non-empty authorization code is required.
func SecretsCallbackHandler(codeCh chan<- string, expectedState string, log *zerolog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		errorParam := r.URL.Query().Get("error")
		errorDesc := r.URL.Query().Get("error_description")

		if errorParam != "" {
			log.Error().Str("error", errorParam).Str("description", errorDesc).Msg("auth error in secrets callback")
			ServeEmbeddedHTML(log, w, PageSecretsError, http.StatusBadRequest)
			return
		}

		if expectedState != "" {
			if st := r.URL.Query().Get("state"); st != expectedState {
				log.Error().Str("got", st).Str("want", expectedState).Msg("invalid state in secrets callback")
				ServeEmbeddedHTML(log, w, PageSecretsError, http.StatusBadRequest)
				return
			}
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			log.Error().Msg("no code in secrets callback")
			ServeEmbeddedHTML(log, w, PageSecretsError, http.StatusBadRequest)
			return
		}

		ServeEmbeddedHTML(log, w, PageSecretsSuccess, http.StatusOK)
		codeCh <- code
	}
}

package oauth

import (
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startCallbackServer starts the real callback server and returns its base URL.
// Port 0 is used so the tests never clash with a CLI already bound to the
// production callback port, and so they can run in parallel.
func startCallbackServer(t *testing.T, expectedState string) (string, chan string) {
	t.Helper()

	log := zerolog.Nop()
	codeCh := make(chan string, 1)

	server, listener, err := NewCallbackHTTPServer("localhost:0", SecretsCallbackHandler(codeCh, expectedState, &log))
	require.NoError(t, err)

	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() { _ = server.Close() })

	return "http://" + listener.Addr().String(), codeCh
}

func getCallback(t *testing.T, url string) *http.Response {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

// The handler writes its response before publishing the code, so the code can
// arrive shortly after the request returns.
func requireCode(t *testing.T, codeCh chan string, want string) {
	t.Helper()

	select {
	case got := <-codeCh:
		assert.Equal(t, want, got)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for authorization code")
	}
}

func requireNoCode(t *testing.T, codeCh chan string) {
	t.Helper()

	select {
	case got := <-codeCh:
		t.Fatalf("expected no authorization code, got %q", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSecretsCallbackServer_acceptsMatchingState(t *testing.T) {
	base, codeCh := startCallbackServer(t, "want-state")

	resp := getCallback(t, base+"/callback?code=the-code&state=want-state")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	requireCode(t, codeCh, "the-code")
}

func TestSecretsCallbackServer_rejects(t *testing.T) {
	tests := []struct {
		name          string
		expectedState string
		query         string
	}{
		{
			name:          "missing state",
			expectedState: "want-state",
			query:         "/callback?code=the-code",
		},
		{
			name:          "mismatched state",
			expectedState: "want-state",
			query:         "/callback?code=the-code&state=other-state",
		},
		{
			name:          "no expected state to compare against",
			expectedState: "",
			query:         "/callback?code=the-code&state=anything",
		},
		{
			name:          "oauth error",
			expectedState: "want-state",
			query:         "/callback?error=access_denied&error_description=denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, codeCh := startCallbackServer(t, tt.expectedState)

			resp := getCallback(t, base+tt.query)

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			requireNoCode(t, codeCh)
		})
	}
}

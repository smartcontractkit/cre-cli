package logout

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func setupCredentialFile(t *testing.T, home string, token string) {
	t.Helper()
	dir := filepath.Join(home, credentials.ConfigDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	path := filepath.Join(dir, credentials.ConfigFile)

	tokens := credentials.CreLoginTokenSet{
		AccessToken:  "dummy-access-token",
		IDToken:      "dummy-id-token",
		RefreshToken: token,
		ExpiresIn:    3600,
		TokenType:    "Bearer",
	}

	data, err := yaml.Marshal(&tokens)
	if err != nil {
		t.Fatalf("failed to marshal token set: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write credentials file: %v", err)
	}
}

func TestExecute_NoCredentialsFile(t *testing.T) {
	tDir := t.TempDir()
	t.Setenv("HOME", tDir)

	creds := credentials.Credentials{
		Tokens:   &credentials.CreLoginTokenSet{},
		APIKey:   "",
		AuthType: "Bearer",
	}

	runtimeCtx := &runtime.Context{
		Logger:         testutil.NewTestLogger(),
		Credentials:    &creds,
		EnvironmentSet: &environments.EnvironmentSet{},
	}
	h := newHandler(runtimeCtx)

	if err := h.execute(); err != nil {
		t.Errorf("expected no error when no creds file, got %v", err)
	}
}

func TestExecute_SuccessRevocationAndRemoval(t *testing.T) {
	tDir := t.TempDir()
	t.Setenv("HOME", tDir)
	token := "test-refresh-token"
	setupCredentialFile(t, tDir, token)

	var received bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		if got := r.Form.Get("token"); got != token {
			t.Errorf("expected token %s, got %s", token, got)
		}
		if got := r.Form.Get("client_id"); got != "fake-client-id" {
			t.Errorf("expected client_id %s, got %s", "fake-client-id", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	revokeURL = ts.URL

	runtimeCtx := &runtime.Context{
		Logger: testutil.NewTestLogger(),
		Credentials: &credentials.Credentials{
			AuthType: credentials.AuthTypeBearer,
			Tokens: &credentials.CreLoginTokenSet{
				RefreshToken: token,
			},
		},
		EnvironmentSet: &environments.EnvironmentSet{
			UIURL:      "https://fake-ui.local",
			AuthBase:   "https://fake-auth0.local",
			ClientID:   "fake-client-id",
			GraphQLURL: "https://fake-graphql.local",
		},
	}

	h := newHandler(runtimeCtx)

	if err := h.execute(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !received {
		t.Error("expected revocation request, but none received")
	}

	credPath := filepath.Join(tDir, credentials.ConfigDir, credentials.ConfigFile)
	if _, err := os.Stat(credPath); !os.IsNotExist(err) {
		t.Errorf("expected credentials file to be removed, but it exists")
	}
}

func TestExecute_RevocationFails_StillRemovesFile(t *testing.T) {
	tDir := t.TempDir()
	t.Setenv("HOME", tDir)
	token := "bad-refresh-token"
	setupCredentialFile(t, tDir, token)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	revokeURL = ts.URL

	runtimeCtx := &runtime.Context{
		Logger: testutil.NewTestLogger(),
		Credentials: &credentials.Credentials{
			AuthType: credentials.AuthTypeBearer,
			Tokens: &credentials.CreLoginTokenSet{
				RefreshToken: token,
			},
		},
		EnvironmentSet: &environments.EnvironmentSet{
			UIURL:      "https://fake-ui.local",
			AuthBase:   "https://fake-auth0.local",
			ClientID:   "fake-client-id",
			GraphQLURL: "https://fake-graphql.local",
		},
	}

	h := newHandler(runtimeCtx)

	if err := h.execute(); err != nil {
		t.Fatalf("expected no error despite revocation failure, got %v", err)
	}

	credPath := filepath.Join(tDir, credentials.ConfigDir, credentials.ConfigFile)
	if _, err := os.Stat(credPath); !os.IsNotExist(err) {
		t.Errorf("expected credentials file to be removed, but it exists")
	}
}

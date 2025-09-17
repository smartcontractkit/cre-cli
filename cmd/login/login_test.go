package login

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
)

func TestSaveCredentials_WritesYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	tokenSet := &credentials.CreLoginTokenSet{
		AccessToken:  "a",
		IDToken:      "i",
		RefreshToken: "r",
		ExpiresIn:    10,
		TokenType:    "Bearer",
	}

	if err := saveCredentials(tokenSet); err != nil {
		t.Fatalf("saveCredentials error: %v", err)
	}

	path := filepath.Join(tmp, credentials.ConfigDir, credentials.ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read config file: %v", err)
	}

	var read credentials.CreLoginTokenSet
	if err := yaml.Unmarshal(data, &read); err != nil {
		t.Fatalf("yaml unmarshal error: %v", err)
	}
	if read.AccessToken != tokenSet.AccessToken ||
		read.IDToken != tokenSet.IDToken ||
		read.RefreshToken != tokenSet.RefreshToken {
		t.Errorf("got %+v; want %+v", read, *tokenSet)
	}
}

func TestCallbackHandler_HTMLResponse(t *testing.T) {
	tmp := t.TempDir()
	successPath := filepath.Join(tmp, "success.html")
	errorPath := filepath.Join(tmp, "error.html")

	if err := os.WriteFile(successPath, []byte("<html>success</html>"), 0600); err != nil {
		t.Fatalf("failed to write success.html: %v", err)
	}
	if err := os.WriteFile(errorPath, []byte("<html>error</html>"), 0600); err != nil {
		t.Fatalf("failed to write error.html: %v", err)
	}

	serveHTML := func(w http.ResponseWriter, path string, statusCode int) {
		w.WriteHeader(statusCode)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", path, err)
		}
		_, _ = w.Write(data)
	}

	callback := func(code string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("code") == "" {
				serveHTML(w, errorPath, http.StatusBadRequest)
				return
			}
			serveHTML(w, successPath, http.StatusOK)
		}
	}

	req1 := httptest.NewRequest(http.MethodGet, "/callback", nil)
	w1 := httptest.NewRecorder()
	callback("")(w1, req1)
	resp1 := w1.Result()
	body1, _ := io.ReadAll(resp1.Body)
	if resp1.StatusCode != http.StatusBadRequest || string(body1) != "<html>error</html>" {
		t.Errorf("missing code: expected error.html, got %s", string(body1))
	}

	req2 := httptest.NewRequest(http.MethodGet, "/callback?code=123", nil)
	w2 := httptest.NewRecorder()
	callback("123")(w2, req2)
	resp2 := w2.Result()
	body2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != http.StatusOK || string(body2) != "<html>success</html>" {
		t.Errorf("valid code: expected success.html, got %s", string(body2))
	}
}

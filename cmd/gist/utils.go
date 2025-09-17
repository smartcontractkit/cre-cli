package gist

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
)

// SetupGitHubAPIMocks configures mock responses for GitHub API endpoints.
// The provided tokens will be treated as valid.
func SetupGitHubAPIMocks(t *testing.T, userName string, validTokens ...string) {
	httpmock.Activate()
	t.Cleanup(httpmock.DeactivateAndReset)
	SetupUserAPIMock(t, userName, validTokens...)
	SetupGistAPIMock(t, validTokens...)
}

// SetupUserAPIMock configures mock responses for GitHub User API.
func SetupUserAPIMock(t *testing.T, userName string, validTokens ...string) {
	httpmock.RegisterResponder("GET", GITHUB_USER_API_URL,
		func(req *http.Request) (*http.Response, error) {
			token := req.Header.Get("Authorization")
			if token == "" {
				return httpmock.NewStringResponse(400, "couldn't find auth token"), nil
			}
			for _, validToken := range validTokens {
				if strings.Contains(token, validToken) {
					return httpmock.NewJsonResponse(200, map[string]interface{}{
						"login": userName,
					})
				}
			}
			return httpmock.NewStringResponse(500, ""), nil
		},
	)
}

// SetupGistAPIMock configures mock responses for GitHub Gist listing API.
func SetupGistAPIMock(t *testing.T, validTokens ...string) {
	httpmock.RegisterResponder("GET", GITHUB_GIST_API_URL,
		func(req *http.Request) (*http.Response, error) {
			token := req.Header.Get("Authorization")
			if token == "" {
				return httpmock.NewStringResponse(400, "couldn't find auth token"), nil
			}
			for _, validToken := range validTokens {
				if strings.Contains(token, validToken) {
					return httpmock.NewStringResponse(200, ""), nil
				}
			}
			return httpmock.NewStringResponse(500, ""), nil
		},
	)
}

// SetupCreateGistAPIMock configures mock responses for GitHub Gist creation API.
func SetupCreateGistAPIMock(t *testing.T, wasmFileName, configFileName string) {
	httpmock.RegisterResponder("POST", GITHUB_GIST_API_URL,
		func(req *http.Request) (*http.Response, error) {
			reqBody := GistRequest{}
			if err := json.NewDecoder(req.Body).Decode(&reqBody); err != nil {
				return httpmock.NewStringResponse(400, "couldn't parse request body"), nil
			}

			if _, ok := reqBody.Files[wasmFileName]; ok {
				return httpmock.NewJsonResponse(200, map[string]interface{}{
					"files": map[string]interface{}{
						wasmFileName: map[string]interface{}{
							"filename": wasmFileName,
							"type":     "application/wasm",
							"raw_url":  "https://gist.githubusercontent.com/user/user_id/raw/file_id/" + wasmFileName,
						},
					},
				})
			}

			if _, ok := reqBody.Files[configFileName]; ok {
				return httpmock.NewJsonResponse(200, map[string]interface{}{
					"files": map[string]interface{}{
						configFileName: map[string]interface{}{
							"filename": configFileName,
							"type":     "text/x-yaml",
							"raw_url":  "https://gist.githubusercontent.com/user/user_id/raw/file_id/" + configFileName,
						},
					},
				})
			}

			return httpmock.NewStringResponse(500, ""), nil
		},
	)
}

package gist

import (
	"encoding/json"

	"github.com/rs/zerolog"
)

type GitHubAPIToken string

func (t GitHubAPIToken) String() string {
	return "*****"
}

func (t GitHubAPIToken) MarshalZerologObject(e *zerolog.Event) {
	e.Str("GitHubAPIToken", "*****")
}

func (t GitHubAPIToken) MarshalJSON() ([]byte, error) {
	return json.Marshal("*****")
}

func (t GitHubAPIToken) RawValue() string {
	return string(t)
}

func HasGistPermissions(token GitHubAPIToken) bool {
	if token == "" {
		return false
	}

	hasPermissions, err := CheckGitHubTokenGistPermissions(token)
	if err != nil {
		return false
	}

	return hasPermissions
}

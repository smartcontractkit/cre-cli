package gist

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/smartcontractkit/dev-platform/internal/logger"
)

func TestGitHubAPIToken_String(t *testing.T) {
	token := GitHubAPIToken("test-token")
	assert.Equal(t, "*****", token.String())
}

func TestGitHubAPIToken_RawValue(t *testing.T) {
	token := GitHubAPIToken("test-token")
	assert.Equal(t, "test-token", token.RawValue())
}

func TestGitHubAPIToken_SprintfHidesSensitiveDetails(t *testing.T) {
	token := GitHubAPIToken("test-token")

	formattedOutput := fmt.Sprintf("token: %s", token)

	assert.Equal(t, "token: *****", formattedOutput)
}

func TestGitHubAPIToken_MarshalZerologObjectHidesSensitiveDetails(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(
		logger.WithOutput(&buf),
		logger.WithLevel("debug"),
		logger.WithConsoleWriter(false),
	)

	log.Info().
		Interface("token", GitHubAPIToken("test-token")).
		Msg("info message")

	assert.Contains(t, buf.String(), "\"token\":{\"GitHubAPIToken\":\"*****\"}")
	assert.NotContains(t, buf.String(), "test-token")
}

func TestGitHubAPIToken_MarshalZerologObjectHidesSensitiveDetailsInStruct(t *testing.T) {
	type SampleInput struct {
		CreateGist     bool
		GitHubAPIToken GitHubAPIToken
	}
	input := SampleInput{
		CreateGist:     true,
		GitHubAPIToken: GitHubAPIToken("test-token"),
	}

	var buf bytes.Buffer
	log := logger.New(
		logger.WithOutput(&buf),
		logger.WithLevel("debug"),
		logger.WithConsoleWriter(false),
	)

	log.Info().
		Interface("inputs", input).
		Msg("info message")

	assert.Contains(t, buf.String(), "\"GitHubAPIToken\":\"*****\"")
	assert.NotContains(t, buf.String(), "test-token")
}

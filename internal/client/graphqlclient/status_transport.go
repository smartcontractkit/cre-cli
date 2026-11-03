package graphqlclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog"
)

const (
	maxErrorBodyBytes  = 64 << 10
	maxLoggedBodyBytes = 512
)

// HTTPStatusError is returned when the CRE API responds with an unhandled non-2xx status.
// Message holds only the GraphQL error message; raw bodies stay out so they never reach telemetry.
type HTTPStatusError struct {
	StatusCode int
	Message    string
}

func (e *HTTPStatusError) Error() string {
	msg := fmt.Sprintf("cre api returned HTTP %d %s", e.StatusCode, http.StatusText(e.StatusCode))
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
}

// statusCheckTransport exists because machinebox/graphql ignores HTTP status codes,
// so gateway error bodies like {"message":"..."} would otherwise decode as empty data.
type statusCheckTransport struct {
	base http.RoundTripper
	log  *zerolog.Logger
}

func (t *statusCheckTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := t.base.RoundTrip(req)
	if err != nil || isSuccess(res.StatusCode) || isFollowableRedirect(res) {
		return res, err
	}
	defer res.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(res.Body, maxErrorBodyBytes))
	if readErr != nil {
		if ctxErr := req.Context().Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, &HTTPStatusError{StatusCode: res.StatusCode}
	}
	t.log.Debug().Int("status", res.StatusCode).Str("body", truncate(body)).Msg("CRE API returned non-2xx response")

	gqlMessage := firstGraphQLErrorMessage(body)
	// Non-transient GraphQL errors (e.g. 401 invalid api key) keep their existing "cre api error" form.
	if gqlMessage != "" && !isRetryableStatus(res.StatusCode) {
		res.Body = io.NopCloser(bytes.NewReader(body))
		return res, nil
	}

	return nil, &HTTPStatusError{StatusCode: res.StatusCode, Message: gqlMessage}
}

func isSuccess(code int) bool {
	return code >= 200 && code < 300
}

// isFollowableRedirect lets http.Client follow redirects, which it does only after the transport returns.
func isFollowableRedirect(res *http.Response) bool {
	switch res.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return res.Header.Get("Location") != ""
	}
	return false
}

func firstGraphQLErrorMessage(body []byte) string {
	var envelope struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &envelope) != nil || len(envelope.Errors) == 0 {
		return ""
	}
	if envelope.Errors[0].Message == "" {
		return "unknown error"
	}
	return envelope.Errors[0].Message
}

func truncate(b []byte) string {
	if len(b) <= maxLoggedBodyBytes {
		return string(b)
	}
	return string(b[:maxLoggedBodyBytes]) + "..."
}

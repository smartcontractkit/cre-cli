package graphqlclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxErrorBodyBytes = 512

// HTTPStatusError is returned when the CRE API responds with a non-2xx status and no GraphQL errors.
type HTTPStatusError struct {
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	msg := fmt.Sprintf("cre api returned HTTP %d %s", e.StatusCode, http.StatusText(e.StatusCode))
	if e.Body != "" {
		msg += ": " + e.Body
	}
	return msg
}

// Retryable reports whether the status indicates a transient server-side condition.
func (e *HTTPStatusError) Retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= http.StatusInternalServerError
}

// statusCheckTransport exists because machinebox/graphql ignores HTTP status codes,
// so gateway error bodies like {"message":"..."} would otherwise decode as empty data.
type statusCheckTransport struct {
	base http.RoundTripper
}

func (t *statusCheckTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := t.base.RoundTrip(req)
	if err != nil || (res.StatusCode >= 200 && res.StatusCode < 300) {
		return res, err
	}
	defer res.Body.Close()

	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return nil, &HTTPStatusError{StatusCode: res.StatusCode}
	}

	// Preserve GraphQL error messages (e.g. 400/401 with an errors array) for the caller.
	if hasGraphQLErrors(body) {
		res.Body = io.NopCloser(bytes.NewReader(body))
		return res, nil
	}

	return nil, &HTTPStatusError{StatusCode: res.StatusCode, Body: truncate(strings.TrimSpace(string(body)))}
}

func hasGraphQLErrors(body []byte) bool {
	var envelope struct {
		Errors []json.RawMessage `json:"errors"`
	}
	return json.Unmarshal(body, &envelope) == nil && len(envelope.Errors) > 0
}

func truncate(s string) string {
	if len(s) <= maxErrorBodyBytes {
		return s
	}
	return s[:maxErrorBodyBytes] + "..."
}

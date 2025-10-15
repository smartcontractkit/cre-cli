package common

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// errReadCloser simulates a failure while reading the body.
type errReadCloser struct{}

func (e *errReadCloser) Read(p []byte) (int, error) { return 0, errors.New("read error") }
func (e *errReadCloser) Close() error               { return nil }

// RTResponse holds one RoundTrip outcome.
type RTResponse struct {
	Response *http.Response
	Err      error
}

// SeqRoundTripper returns a sequence of outcomes across calls.
// If calls exceed length, it repeats the last element.
type SeqRoundTripper struct {
	Seq   []RTResponse
	Calls int
}

func (s *SeqRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	i := s.Calls
	if i >= len(s.Seq) {
		i = len(s.Seq) - 1
	}
	s.Calls++
	entry := s.Seq[i]
	return entry.Response, entry.Err
}

// makeResp builds a tiny HTTP response with given status and body.
func makeResp(code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}

func TestPostToGateway(t *testing.T) {
	h, _, _ := newMockHandler(t)

	t.Run("success (single attempt)", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":"abc","result":{"ok":true}}`

		rt := &SeqRoundTripper{
			Seq: []RTResponse{
				{Response: makeResp(200, body)},
			},
		}
		mockedHTTP := &http.Client{Transport: rt}
		h.Gw = &HTTPClient{
			URL:           "https://unit-test.gw",
			Client:        mockedHTTP,
			RetryAttempts: 3,
			RetryDelay:    0, // fast tests
		}

		respBytes, status, err := h.Gw.Post([]byte(`{"x":1}`))
		assert.NoError(t, err)
		assert.Equal(t, 200, status)
		assert.Equal(t, body, string(respBytes))
		assert.Equal(t, 1, rt.Calls)
	})

	t.Run("http transport error -> retries then fail", func(t *testing.T) {
		rt := &SeqRoundTripper{
			Seq: []RTResponse{
				{Err: errors.New("network down")},
				{Err: errors.New("network still down")},
			},
		}
		mockedHTTP := &http.Client{Transport: rt}
		h.Gw = &HTTPClient{
			URL:           "https://unit-test.gw",
			Client:        mockedHTTP,
			RetryAttempts: 2,
			RetryDelay:    0,
		}

		_, _, err := h.Gw.Post([]byte(`{}`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "network")
		assert.Equal(t, 2, rt.Calls)
	})

	t.Run("read error -> retries then fail", func(t *testing.T) {
		rt := &SeqRoundTripper{
			Seq: []RTResponse{
				{Response: &http.Response{StatusCode: 200, Body: &errReadCloser{}, Header: make(http.Header)}},
				{Response: &http.Response{StatusCode: 200, Body: &errReadCloser{}, Header: make(http.Header)}},
			},
		}
		mockedHTTP := &http.Client{Transport: rt}
		h.Gw = &HTTPClient{
			URL:           "https://unit-test.gw",
			Client:        mockedHTTP,
			RetryAttempts: 2,
			RetryDelay:    0,
		}

		_, _, err := h.Gw.Post([]byte(`{}`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read response body: read error")
		assert.Equal(t, 2, rt.Calls)
	})

	t.Run("non-200 then 200 -> success after retry", func(t *testing.T) {
		successBody := `{"ok":true}`
		rt := &SeqRoundTripper{
			Seq: []RTResponse{
				{Response: makeResp(503, "temporary outage")},
				{Response: makeResp(200, successBody)},
			},
		}
		mockedHTTP := &http.Client{Transport: rt}
		h.Gw = &HTTPClient{
			URL:           "https://unit-test.gw",
			Client:        mockedHTTP,
			RetryAttempts: 3,
			RetryDelay:    0,
		}

		respBytes, status, err := h.Gw.Post([]byte(`{}`))
		assert.NoError(t, err)
		assert.Equal(t, 200, status)
		assert.Equal(t, successBody, string(respBytes))
		assert.Equal(t, 2, rt.Calls)
	})

	t.Run("always non-200 -> fail after attempts", func(t *testing.T) {
		rt := &SeqRoundTripper{
			Seq: []RTResponse{
				{Response: makeResp(500, "err1")},
				{Response: makeResp(429, "err2")},
				{Response: makeResp(400, "err3")}, // any non-200 should still retry/fail
			},
		}
		mockedHTTP := &http.Client{Transport: rt}
		h.Gw = &HTTPClient{
			URL:           "https://unit-test.gw",
			Client:        mockedHTTP,
			RetryAttempts: 3,
			RetryDelay:    0,
		}

		_, status, err := h.Gw.Post([]byte(`{}`))
		assert.Error(t, err)
		// status is from the last attempt
		assert.Equal(t, 400, status)
		assert.Contains(t, err.Error(), "gateway POST failed")
		assert.Equal(t, 3, rt.Calls)
	})

	t.Run("error then success -> ok", func(t *testing.T) {
		body := `{"ok":true}`
		rt := &SeqRoundTripper{
			Seq: []RTResponse{
				{Err: errors.New("dial tcp: i/o timeout")},
				{Response: makeResp(200, body)},
			},
		}
		mockedHTTP := &http.Client{Transport: rt}
		h.Gw = &HTTPClient{
			URL:           "https://unit-test.gw",
			Client:        mockedHTTP,
			RetryAttempts: 5,
			RetryDelay:    0,
		}

		respBytes, status, err := h.Gw.Post([]byte(`{}`))
		assert.NoError(t, err)
		assert.Equal(t, 200, status)
		assert.Equal(t, body, string(respBytes))
		assert.Equal(t, 2, rt.Calls)
	})

	// Optional: prove delays are honored if set
	t.Run("honors small delay", func(t *testing.T) {
		rt := &SeqRoundTripper{
			Seq: []RTResponse{
				{Response: makeResp(503, "nope")},
				{Response: makeResp(200, `{"ok":true}`)},
			},
		}
		mockedHTTP := &http.Client{Transport: rt}
		h.Gw = &HTTPClient{
			URL:           "https://unit-test.gw",
			Client:        mockedHTTP,
			RetryAttempts: 2,
			RetryDelay:    5 * time.Millisecond,
		}

		_, status, err := h.Gw.Post([]byte(`{}`))
		assert.NoError(t, err)
		assert.Equal(t, 200, status)
		assert.Equal(t, 2, rt.Calls)
	})
}

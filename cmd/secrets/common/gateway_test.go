package common

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

		_, status, err := h.Gw.Post([]byte(`{}`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gateway request failed")
		assert.Equal(t, 0, status)   // last attempt was a transport error -> status 0
		assert.Equal(t, 2, rt.Calls) // retried
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

		_, status, err := h.Gw.Post([]byte(`{}`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read response body: read error")
		assert.Equal(t, 200, status) // postOnce had a resp with 200 but read failed
		assert.Equal(t, 2, rt.Calls) // retried
	})

	t.Run("non-200 containing allowlist then 200 -> success after retry", func(t *testing.T) {
		successBody := `{"ok":true}`
		rt := &SeqRoundTripper{
			Seq: []RTResponse{
				{Response: makeResp(503, "Request not allowlisted - pending")},
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

	t.Run("always non-200 with allowlist hint -> retry until attempts then fail", func(t *testing.T) {
		rt := &SeqRoundTripper{
			Seq: []RTResponse{
				{Response: makeResp(500, "request not allowlisted (booting up)")},
				{Response: makeResp(429, "REQUEST NOT ALLOWLISTED - still syncing")},
				{Response: makeResp(429, "request not allowlisted - keep waiting")},
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
		assert.Equal(t, 429, status) // from last attempt
		assert.Contains(t, err.Error(), "gateway POST failed")
		assert.Equal(t, 3, rt.Calls)
	})

	t.Run("transport error then success -> ok after retry", func(t *testing.T) {
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
		assert.Equal(t, 2, rt.Calls) // retried and succeeded
	})
}

func TestPostWithBearer(t *testing.T) {
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		assert.Equal(t, "application/jsonrpc", r.Header.Get("Content-Type"))
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"x","result":{}}`))
	}))
	t.Cleanup(srv.Close)

	g := &HTTPClient{
		URL:           srv.URL,
		Client:        srv.Client(),
		RetryAttempts: 1,
		RetryDelay:    0,
	}
	body := []byte(`{"jsonrpc":"2.0","id":"1","method":"test","params":{}}`)
	resp, status, err := g.PostWithBearer(body, "my-jwt")
	require.NoError(t, err)
	assert.Equal(t, 200, status)
	assert.Contains(t, string(resp), "jsonrpc")
	assert.Equal(t, "Bearer my-jwt", sawAuth)
}

func TestPostWithBearer_EmptyToken(t *testing.T) {
	g := &HTTPClient{URL: "http://example.com", Client: http.DefaultClient}
	_, _, err := g.PostWithBearer([]byte(`{}`), "   ")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty bearer token")
}

func TestPostWithBearer_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	g := &HTTPClient{
		URL:           srv.URL,
		Client:        srv.Client(),
		RetryAttempts: 3,
		RetryDelay:    0,
	}
	_, status, err := g.PostWithBearer([]byte(`{}`), "tok")
	assert.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Contains(t, err.Error(), "non-200")
}

func TestPostWithBearer_TransportErrorThenSuccess(t *testing.T) {
	body := `{"ok":true}`
	rt := &SeqRoundTripper{
		Seq: []RTResponse{
			{Err: errors.New("connection reset")},
			{Response: makeResp(200, body)},
		},
	}
	g := &HTTPClient{
		URL:           "https://unit-test.gw",
		Client:        &http.Client{Transport: rt},
		RetryAttempts: 3,
		RetryDelay:    0,
	}
	respBytes, status, err := g.PostWithBearer([]byte(`{}`), "jwt")
	assert.NoError(t, err)
	assert.Equal(t, 200, status)
	assert.Equal(t, body, string(respBytes))
	assert.Equal(t, 2, rt.Calls)
}

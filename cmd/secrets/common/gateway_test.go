package common

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MockRoundTripper struct {
	Response *http.Response
	Err      error
}

func (mrt *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return mrt.Response, mrt.Err
}

type errReadCloser struct{}

func (e *errReadCloser) Read(p []byte) (int, error) { return 0, errors.New("read error") }
func (e *errReadCloser) Close() error               { return nil }

func TestPostToGateway(t *testing.T) {
	h, _, _ := newMockHandler(t)

	t.Run("success", func(t *testing.T) {
		mockResponseBody := `{"jsonrpc":"2.0","id":"abc","result":{"ok":true}}`
		mockResponse := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(mockResponseBody)),
			Header:     make(http.Header),
		}

		// Wire mock transport into Handler's HTTP client
		mockedHttpClient := &http.Client{Transport: &MockRoundTripper{Response: mockResponse}}
		h.Gw = &HTTPClient{URL: "https://unit-test.gw", Client: mockedHttpClient}

		respBytes, status, err := h.Gw.Post([]byte(`{"x":1}`))
		assert.NoError(t, err)
		assert.Equal(t, 200, status)
		assert.Equal(t, mockResponseBody, string(respBytes))
	})

	t.Run("http error", func(t *testing.T) {
		mockedHttpClient := &http.Client{Transport: &MockRoundTripper{Response: nil, Err: errors.New("network down")}}
		h.Gw = &HTTPClient{URL: "https://unit-test.gw", Client: mockedHttpClient}

		_, _, err := h.Gw.Post([]byte(`{}`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "network down")
	})

	t.Run("read error", func(t *testing.T) {
		mockResponse := &http.Response{
			StatusCode: 200,
			Body:       &errReadCloser{},
			Header:     make(http.Header),
		}
		mockedHttpClient := &http.Client{Transport: &MockRoundTripper{Response: mockResponse}}
		h.Gw = &HTTPClient{URL: "https://unit-test.gw", Client: mockedHttpClient}

		_, _, err := h.Gw.Post([]byte(`{}`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read response body: read error")
	})
}

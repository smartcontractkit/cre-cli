package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{"https://example.com/binary.wasm", true},
		{"http://example.com/binary.wasm", true},
		{"HTTP://EXAMPLE.COM", false},
		{"./local/path.wasm", false},
		{"/absolute/path.wasm", false},
		{"", false},
		{"ftp://example.com", false},
		{"https://", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, IsURL(tt.input))
		})
	}
}

func TestFetchURL(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		body := []byte("hello world")
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		}))
		defer srv.Close()

		data, err := FetchURL(srv.URL)
		require.NoError(t, err)
		assert.Equal(t, body, data)
	})

	t.Run("non-200 status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		_, err := FetchURL(srv.URL)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "returned status 404")
	})

	t.Run("unreachable host", func(t *testing.T) {
		_, err := FetchURL("http://127.0.0.1:1")
		require.Error(t, err)
	})
}

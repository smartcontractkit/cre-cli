package graphqlclient

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"testing"
)

func TestIsRetryable(t *testing.T) {
	urlErr := func(err error) error { return &url.Error{Op: "Post", URL: "https://api", Err: err} }

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"503", &HTTPStatusError{StatusCode: http.StatusServiceUnavailable}, true},
		{"429", &HTTPStatusError{StatusCode: http.StatusTooManyRequests}, true},
		{"401", &HTTPStatusError{StatusCode: http.StatusUnauthorized}, false},
		{"wrapped 502", fmt.Errorf("auth: %w", &HTTPStatusError{StatusCode: http.StatusBadGateway}), true},
		{"connection reset", urlErr(&net.OpError{Op: "read", Err: syscall.ECONNRESET}), true},
		{"connection refused", urlErr(&net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}), true},
		{"eof", urlErr(io.EOF), true},
		{"temporary dns", urlErr(&net.DNSError{IsTemporary: true}), true},
		{"permanent dns", urlErr(&net.DNSError{IsNotFound: true}), false},
		{"tls verification", urlErr(x509.UnknownAuthorityError{}), false},
		{"context canceled", urlErr(context.Canceled), false},
		{"context deadline", urlErr(context.DeadlineExceeded), false},
		{"token refresh timeout", fmt.Errorf("%w: %w", ErrTokenRefresh, urlErr(&net.OpError{Op: "read", Err: syscall.ECONNRESET})), false},
		{"graphql error", errors.New("cre api error: invalid api key"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"401", &HTTPStatusError{StatusCode: http.StatusUnauthorized}, true},
		{"wrapped 403", fmt.Errorf("auth: %w", &HTTPStatusError{StatusCode: http.StatusForbidden}), true},
		{"503", &HTTPStatusError{StatusCode: http.StatusServiceUnavailable}, false},
		{"graphql error", errors.New("cre api error: invalid api key"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthError(tt.err); got != tt.want {
				t.Errorf("IsAuthError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

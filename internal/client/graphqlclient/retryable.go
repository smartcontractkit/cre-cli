package graphqlclient

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
)

// IsRetryable reports whether a failed request is worth repeating.
// Context errors are excluded so callers' cancellations and deadlines are honoured.
func IsRetryable(err error) bool {
	if err == nil || errors.Is(err, ErrTokenRefresh) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) {
		return isRetryableStatus(statusErr.StatusCode)
	}

	return isTransientNetworkError(err)
}

// Permanent failures like TLS verification or bad URLs are excluded to avoid pointless retries.
func isTransientNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsTemporary {
		return true
	}
	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED)
}

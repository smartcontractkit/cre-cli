package rpc

import (
	"fmt"
	"net"
	"net/url"
)

// CleartextPolicyOptions controls cleartext RPC policy evaluation.
type CleartextPolicyOptions struct {
	AllowInsecure bool
}

// IsLoopbackHost reports whether host is a local loopback address (localhost, 127.0.0.1, ::1).
func IsLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// EvaluateCleartextRPC blocks non-localhost HTTP RPC URLs unless explicitly opted in.
// Returns a warning when cleartext use is allowed via --allow-insecure-rpc.
func EvaluateCleartextRPC(rpcURL string, opts CleartextPolicyOptions) (warnMsg string, blockErr error) {
	if err := IsValidURL(rpcURL); err != nil {
		return "", err
	}

	parsed, err := url.Parse(rpcURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse RPC URL: invalid format")
	}

	if parsed.Scheme == "https" {
		return "", nil
	}
	if parsed.Scheme != "http" {
		return "", nil
	}
	if IsLoopbackHost(parsed.Hostname()) {
		return "", nil
	}

	redacted := RedactURL(rpcURL)
	if !opts.AllowInsecure {
		return "", fmt.Errorf(
			"cleartext RPC URL %q is not allowed; use https:// or pass --allow-insecure-rpc to opt in",
			redacted,
		)
	}

	return fmt.Sprintf(
		"using cleartext HTTP RPC URL %q; this is insecure even with --allow-insecure-rpc",
		redacted,
	), nil
}

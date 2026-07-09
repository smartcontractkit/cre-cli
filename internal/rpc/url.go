package rpc

import (
	"fmt"
	"net/url"
)

// IsValidURL checks that rpcURL has an http or https scheme and a non-empty host.
func IsValidURL(rpcURL string) error {
	parsedURL, err := url.Parse(rpcURL)
	if err != nil {
		return fmt.Errorf("failed to parse RPC URL: invalid format")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid scheme in RPC URL: %s", parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("invalid host in RPC URL: %s", parsedURL.Host)
	}

	return nil
}

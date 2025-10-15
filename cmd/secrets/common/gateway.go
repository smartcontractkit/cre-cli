package common

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/avast/retry-go/v4"
)

type GatewayClient interface {
	Post(body []byte) (respBody []byte, status int, err error)
}

type HTTPClient struct {
	URL           string
	Client        *http.Client
	RetryAttempts uint
	RetryDelay    time.Duration
}

func (g *HTTPClient) Post(body []byte) ([]byte, int, error) {
	attempts := g.RetryAttempts
	if attempts == 0 {
		attempts = 5
	}
	delay := g.RetryDelay
	if delay == 0 {
		delay = 3 * time.Second
	}

	var respBody []byte
	var status int

	err := retry.Do(
		func() error {
			b, s, e := g.postOnce(body)
			respBody, status = b, s
			if e != nil {
				return e // retry on any error
			}
			if s != http.StatusOK {
				return fmt.Errorf("gateway returned non-200: %d", s) // retry on any non-200
			}
			return nil // success
		},
		retry.Attempts(attempts),
		retry.Delay(delay),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			fmt.Printf("Waiting for block confirmation and retrying gateway POST (attempt %d/%d): %v", n+1, attempts, err)
		}),
	)

	if err != nil {
		// Return the last seen body/status to aid debugging.
		return respBody, status, fmt.Errorf("gateway POST failed after %d attempts: %w", attempts, err)
	}
	return respBody, status, nil
}

func (g *HTTPClient) postOnce(body []byte) ([]byte, int, error) {
	req, err := http.NewRequest("POST", g.URL, bytes.NewBuffer(body))
	if err != nil {
		return nil, 0, fmt.Errorf("create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/jsonrpc")
	req.Header.Set("Accept", "application/json")

	if g.Client == nil {
		return nil, 0, fmt.Errorf("HTTP client is not initialized")
	}

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("HTTP request to gateway failed: %w", err)
	}
	defer resp.Body.Close()

	b, rerr := io.ReadAll(resp.Body)
	if rerr != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", rerr)
	}
	return b, resp.StatusCode, nil
}

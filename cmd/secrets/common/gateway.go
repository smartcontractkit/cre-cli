package common

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/avast/retry-go/v4"

	"github.com/smartcontractkit/cre-cli/internal/ui"
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
		delay = 4 * time.Second
	}

	var respBody []byte
	var status int

	err := retry.Do(
		func() error {
			b, s, e := g.postOnce(body)
			respBody, status = b, s

			// 1) If transport error -> retry
			if e != nil {
				return fmt.Errorf("gateway request failed: %w", e) // retry-go will retry
			}

			// 2) If non-200 and body contains "not allowlisted" -> retry
			if s != http.StatusOK {
				lower := bytes.ToLower(b)
				if bytes.Contains(lower, []byte("request not allowlisted")) {
					return fmt.Errorf("gateway not allowlisted yet (status=%d)", s)
				}
				// 3) Any other non-200 -> no retry
				return retry.Unrecoverable(fmt.Errorf("gateway returned non-200 (no allowlist hint): %d", s))
			}

			// 4) Success
			return nil
		},
		retry.Attempts(uint(attempts)),
		retry.Delay(delay),
		retry.LastErrorOnly(true),
		retry.OnRetry(func(n uint, err error) {
			ui.Dim(fmt.Sprintf("Waiting for on-chain allowlist finalization... (attempt %d/%d): %v", n+1, attempts, err))
		}),
	)

	if err != nil {
		// Return the last seen body/status to aid debugging.
		return respBody, status, fmt.Errorf("gateway POST failed: %w", err)
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

	resp, err := g.Client.Do(req) // #nosec G704 -- URL is from trusted CLI configuration
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

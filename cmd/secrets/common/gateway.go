package common

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

type GatewayClient interface {
	Post(body []byte) (respBody []byte, status int, err error)
}

type HTTPClient struct {
	URL    string
	Client *http.Client
}

func (g *HTTPClient) Post(body []byte) ([]byte, int, error) {
	req, err := http.NewRequest("POST", g.URL, bytes.NewBuffer(body))
	if err != nil {
		return nil, 0, fmt.Errorf("create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/jsonrpc")
	req.Header.Set("Accept", "application/json")

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

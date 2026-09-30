package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
)

func TestParseGatewayResponse(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		postErr    error
		wantResult string
		wantCode   int64
		wantMsg    string
	}{
		{
			name:       "success with result",
			body:       `{"jsonrpc":"2.0","id":"r1","method":"workflows.execute","result":{"status":"ACCEPTED","workflow_execution_id":"0xabc"}}`,
			wantResult: `{"status":"ACCEPTED","workflow_execution_id":"0xabc"}`,
		},
		{
			name: "success without result",
			body: `{"jsonrpc":"2.0","id":"r1"}`,
		},
		{
			name:     "JSON-RPC error on HTTP 200",
			body:     `{"jsonrpc":"2.0","id":"r1","error":{"code":-32600,"message":"Workflow not found"}}`,
			wantCode: -32600,
			wantMsg:  "Workflow not found",
		},
		{
			name:     "JSON-RPC error carried by a non-200 status",
			body:     `{"jsonrpc":"2.0","id":"r1","error":{"code":-32600,"message":"Auth failure"}}`,
			postErr:  errors.New("gateway POST failed: gateway returned non-200: status_code=400"),
			wantCode: -32600,
			wantMsg:  "Auth failure",
		},
		{
			name:     "transport error",
			postErr:  errors.New("HTTP request to gateway failed: connection refused"),
			wantCode: jsonrpc2.ErrUnknown,
			wantMsg:  "connection refused",
		},
		{
			name:     "non-JSON-RPC error page",
			body:     `<html>bad gateway</html>`,
			postErr:  errors.New("gateway returned non-200: status_code=502, body=<html>bad gateway</html>"),
			wantCode: jsonrpc2.ErrUnknown,
			wantMsg:  "status_code=502",
		},
		{
			name:     "HTTP 200 that is not JSON-RPC",
			body:     `{"hello":"world"}`,
			wantCode: jsonrpc2.ErrUnknown,
			wantMsg:  "not JSON-RPC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, rpcErr := parseGatewayResponse([]byte(tt.body), tt.postErr)

			if tt.wantMsg == "" {
				require.Nil(t, rpcErr)
				if tt.wantResult == "" {
					assert.Nil(t, result)
				} else {
					assert.JSONEq(t, tt.wantResult, string(result))
				}
				return
			}
			require.NotNil(t, rpcErr)
			assert.Nil(t, result)
			assert.Equal(t, tt.wantCode, rpcErr.Code)
			assert.Contains(t, rpcErr.Message, tt.wantMsg)
		})
	}
}

func TestParseGatewayResponse_TruncatesHugeBodies(t *testing.T) {
	_, rpcErr := parseGatewayResponse([]byte(strings.Repeat("x", 10_000)), nil)

	require.NotNil(t, rpcErr)
	assert.Less(t, len(rpcErr.Message), 1_000)
}

func TestNewOutput_Status(t *testing.T) {
	t.Run("defaults to ACCEPTED", func(t *testing.T) {
		assert.Equal(t, "ACCEPTED", newOutput("0x1", "r", "u", json.RawMessage(`{}`), nil).Status)
		assert.Equal(t, "ACCEPTED", newOutput("0x1", "r", "u", nil, nil).Status)
	})
	t.Run("uses the node-reported status", func(t *testing.T) {
		assert.Equal(t, "QUEUED", newOutput("0x1", "r", "u", json.RawMessage(`{"status":"QUEUED"}`), nil).Status)
	})
	t.Run("ERROR on JSON-RPC error", func(t *testing.T) {
		out := newOutput("0x1", "r", "u", nil, &jsonrpc2.WireError{Code: -32603, Message: "boom"})
		assert.Equal(t, "ERROR", out.Status)
	})
}

func TestPrintJSON_Shape(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, printJSON(&buf, newOutput("0xabc", "req-1", "https://gw.example", nil, nil)))

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, map[string]any{
		"workflowId": "0xabc",
		"requestId":  "req-1",
		"gatewayUrl": "https://gw.example",
		"status":     "ACCEPTED",
		"result":     nil,
	}, got)
}

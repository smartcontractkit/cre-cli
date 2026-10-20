package http

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	gwtypes "github.com/smartcontractkit/chainlink-common/pkg/types/gateway"

	"github.com/smartcontractkit/cre-cli/internal/ui"
)

const (
	statusError        = "ERROR"
	maxErrorBodyLength = 512
)

// output is the machine-readable result printed to stdout with --json.
type output struct {
	WorkflowID string              `json:"workflowId"`
	RequestID  string              `json:"requestId"`
	GatewayURL string              `json:"gatewayUrl"`
	Status     string              `json:"status"`
	Result     json.RawMessage     `json:"result"`
	Error      *jsonrpc2.WireError `json:"error,omitempty"`
}

// parseGatewayResponse maps the raw POST outcome to a JSON-RPC result or error. The Gateway
// returns its JSON-RPC errors with non-200 HTTP statuses, so the body is inspected first.
func parseGatewayResponse(body []byte, postErr error) (json.RawMessage, *jsonrpc2.WireError) {
	var resp jsonrpc2.Response[json.RawMessage]
	if err := json.Unmarshal(body, &resp); err == nil && resp.Version == jsonrpc2.JsonRpcVersion {
		if resp.Error != nil {
			return nil, resp.Error
		}
		if postErr == nil {
			if resp.Result == nil {
				return nil, nil
			}
			return *resp.Result, nil
		}
	}

	if postErr != nil {
		return nil, &jsonrpc2.WireError{Code: jsonrpc2.ErrUnknown, Message: truncate(postErr.Error(), maxErrorBodyLength)}
	}
	return nil, &jsonrpc2.WireError{
		Code:    jsonrpc2.ErrUnknown,
		Message: "gateway returned a response that is not JSON-RPC: " + truncate(string(body), maxErrorBodyLength),
	}
}

func newOutput(workflowID, requestID, gatewayURL string, result json.RawMessage, rpcErr *jsonrpc2.WireError) output {
	out := output{
		WorkflowID: workflowID,
		RequestID:  requestID,
		GatewayURL: gatewayURL,
		Status:     resultStatus(result),
		Result:     result,
		Error:      rpcErr,
	}
	if rpcErr != nil {
		out.Status = statusError
	}
	return out
}

// resultStatus prefers the status reported by the workflow node and assumes ACCEPTED otherwise,
// since a JSON-RPC success from the Gateway means the trigger was delivered.
func resultStatus(result json.RawMessage) string {
	var parsed struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &parsed); err == nil && parsed.Status != "" {
		return parsed.Status
	}
	return string(gwtypes.HTTPTriggerStatusAccepted)
}

func printJSON(w io.Writer, out output) error {
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode output: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func printSummary(out output) {
	ui.Line()
	ui.Success(fmt.Sprintf("Trigger request sent (status: %s)", out.Status))
	ui.Line()
	ui.Bold("Details:")
	ui.Dim(fmt.Sprintf("   Workflow ID: %s", out.WorkflowID))
	ui.Dim(fmt.Sprintf("   Request ID:  %s", out.RequestID))
	ui.Dim(fmt.Sprintf("   Gateway:     %s", out.GatewayURL))
	if len(out.Result) > 0 {
		ui.Dim(fmt.Sprintf("   Result:      %s", string(out.Result)))
	}
	ui.Line()
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}

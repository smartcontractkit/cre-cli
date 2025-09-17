package common

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"
)

type testRPCResp struct {
	JSONRPC string         `json:"jsonrpc,omitempty"`
	ID      string         `json:"id,omitempty"`
	Result  *testRPCResult `json:"result,omitempty"`
	Error   *testRPCError  `json:"error,omitempty"`
}
type testRPCResult struct {
	Payload json.RawMessage `json:"payload,omitempty"` // embed raw JSON
}
type testRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func encodeRPCBodyFromPayload(payload []byte) []byte {
	resp := testRPCResp{
		JSONRPC: "2.0",
		ID:      "1",
		Result:  &testRPCResult{Payload: json.RawMessage(payload)},
	}
	b, _ := json.Marshal(resp)
	return b
}

func newTestHandler(buf *bytes.Buffer) *Handler {
	logger := zerolog.New(buf)
	return &Handler{Log: &logger}
}

// Build the payload using the real proto types
func buildCreatePayloadProto(t *testing.T) []byte {
	t.Helper()
	msg := &vault.CreateSecretsResponse{
		Responses: []*vault.CreateSecretResponse{
			{
				Id:      &vault.SecretIdentifier{Key: "k1", Owner: "o1", Namespace: "n1"},
				Success: true,
			},
			{
				Id:      &vault.SecretIdentifier{Key: "k2", Owner: "o2", Namespace: "n2"},
				Success: false,
				Error:   "boom",
			},
			{
				// nil Id on purpose, Success = true
				Success: true,
			},
		},
	}
	b, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("protojson.Marshal failed: %v", err)
	}
	return b
}

func buildUpdatePayloadProto(t *testing.T) []byte {
	t.Helper()
	msg := &vault.UpdateSecretsResponse{
		Responses: []*vault.UpdateSecretResponse{
			{
				Id:      &vault.SecretIdentifier{Key: "ku", Owner: "ou", Namespace: "nu"},
				Success: true,
			},
		},
	}
	b, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("protojson.Marshal failed: %v", err)
	}
	return b
}

func buildDeletePayloadProto(t *testing.T) []byte {
	t.Helper()
	msg := &vault.DeleteSecretsResponse{
		Responses: []*vault.DeleteSecretResponse{
			{
				Id:      &vault.SecretIdentifier{Key: "kd", Owner: "od", Namespace: "nd"},
				Success: true,
			},
		},
	}
	b, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("protojson.Marshal failed: %v", err)
	}
	return b
}

// JSON-RPC error envelope
func encodeRPCBodyFromError(code int, msg string) []byte {
	resp := testRPCResp{
		JSONRPC: "2.0",
		ID:      "1",
		Error:   &testRPCError{Code: code, Message: msg},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestParseVaultGatewayResponse_Create_LogsPerItem(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	body := encodeRPCBodyFromPayload(buildCreatePayloadProto(t))
	if err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsCreate, body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Expect 2 successes + 1 failure
	if got := strings.Count(out, `"message":"secret created"`); got < 2 {
		t.Fatalf("expected at least 2 'secret created' logs, got %d.\nlogs:\n%s", got, out)
	}
	if got := strings.Count(out, `"message":"secret create failed"`); got != 1 {
		t.Fatalf("expected 1 'secret create failed' log, got %d.\nlogs:\n%s", got, out)
	}
	// Spot-check structured fields
	if !strings.Contains(out, `"secret_id":"k1"`) || !strings.Contains(out, `"namespace":"n1"`) || !strings.Contains(out, `"owner":"o1"`) {
		t.Fatalf("expected id/owner/namespace fields for first secret in logs, got:\n%s", out)
	}
	if !strings.Contains(out, `"error":"boom"`) {
		t.Fatalf("expected error text to be logged for failed item, got:\n%s", out)
	}
}

func TestParseVaultGatewayResponse_Update_Success(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	body := encodeRPCBodyFromPayload(buildUpdatePayloadProto(t))
	if err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsUpdate, body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"message":"secret updated"`) {
		t.Fatalf("expected 'secret updated' log, got:\n%s", out)
	}
	if !strings.Contains(out, `"secret_id":"ku"`) ||
		!strings.Contains(out, `"owner":"ou"`) ||
		!strings.Contains(out, `"namespace":"nu"`) {
		t.Fatalf("expected id/owner/namespace in logs, got:\n%s", out)
	}
}

func TestParseVaultGatewayResponse_Delete_Success(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	body := encodeRPCBodyFromPayload(buildDeletePayloadProto(t))
	if err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsDelete, body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"message":"secret deleted"`) {
		t.Fatalf("expected 'secret deleted' log, got:\n%s", out)
	}
	if !strings.Contains(out, `"secret_id":"kd"`) ||
		!strings.Contains(out, `"owner":"od"`) ||
		!strings.Contains(out, `"namespace":"nd"`) {
		t.Fatalf("expected id/owner/namespace in logs, got:\n%s", out)
	}
}

func TestParseVaultGatewayResponse_JSONRPCError(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	body := encodeRPCBodyFromError(-32000, "upstream failed")
	err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsCreate, body)
	if err == nil || !strings.Contains(err.Error(), "gateway returned JSON-RPC error") ||
		!strings.Contains(err.Error(), "upstream failed") {
		t.Fatalf("expected JSON-RPC error surfaced, got: %v", err)
	}
}

func TestParseVaultGatewayResponse_EmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	// Omit payload entirely -> handler should report "empty SignedOCRResponse payload"
	raw := encodeRPCBodyFromPayload(nil)
	err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsUpdate, raw)
	if err == nil || !strings.Contains(err.Error(), "empty SignedOCRResponse payload") {
		t.Fatalf("expected empty payload error, got: %v", err)
	}
}

func TestParseVaultGatewayResponse_MalformedTopLevelJSON(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	raw := []byte(`{"jsonrpc":"2.0","id":"1","result": this is not valid}`)
	err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsUpdate, raw)
	if err == nil || !strings.Contains(err.Error(), "failed to unmarshal JSON-RPC response") {
		t.Fatalf("expected unmarshal error, got: %v", err)
	}
}

func TestParseVaultGatewayResponse_BadPayloadForCreate(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	// Wrong shape for the proto: responses should be an array.
	raw := encodeRPCBodyFromPayload([]byte(`{"responses":"not-an-array"}`))
	err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsCreate, raw)
	if err == nil || !strings.Contains(err.Error(), "failed to decode create payload") {
		t.Fatalf("expected proto decode error for create, got: %v", err)
	}
}

func TestParseVaultGatewayResponse_UnsupportedMethod_Warns(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	// Non-empty payload so it passes "empty payload" check; method is unknown -> warn.
	raw := encodeRPCBodyFromPayload([]byte(`{"anything":"ok"}`))
	if err := h.ParseVaultGatewayResponse("totally.unknown.method", raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"level":"warn"`) ||
		!strings.Contains(out, `"received response for unsupported method; skipping payload decode"`) {
		t.Fatalf("expected warn log for unsupported method, got:\n%s", out)
	}
}

func buildListPayloadProtoSuccessWithItems(t *testing.T) []byte {
	t.Helper()
	msg := &vault.ListSecretIdentifiersResponse{
		Identifiers: []*vault.SecretIdentifier{
			{Key: "l1", Owner: "ol1", Namespace: "nl1"},
			{Key: "l2", Owner: "ol2", Namespace: "nl2"},
		},
		Success: true,
	}
	b, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("protojson.Marshal failed: %v", err)
	}
	return b
}

func buildListPayloadProtoEmptySuccess(t *testing.T) []byte {
	t.Helper()
	msg := &vault.ListSecretIdentifiersResponse{
		Identifiers: nil,
		Success:     true,
	}
	b, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("protojson.Marshal failed: %v", err)
	}
	return b
}

func buildListPayloadProtoFailure(t *testing.T, errMsg string) []byte {
	t.Helper()
	msg := &vault.ListSecretIdentifiersResponse{
		Identifiers: nil, // could be empty or non-empty; handler logs a single error line either way
		Success:     false,
		Error:       errMsg,
	}
	b, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatalf("protojson.Marshal failed: %v", err)
	}
	return b
}

func TestParseVaultGatewayResponse_List_SuccessWithIdentifiers(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	body := encodeRPCBodyFromPayload(buildListPayloadProtoSuccessWithItems(t))
	if err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsList, body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Two identifiers -> two info lines
	if got := strings.Count(out, `"message":"secret identifier"`); got != 2 {
		t.Fatalf("expected 2 'secret identifier' logs, got %d.\nlogs:\n%s", got, out)
	}
	// Spot-check fields
	if !strings.Contains(out, `"secret_id":"l1"`) || !strings.Contains(out, `"owner":"ol1"`) || !strings.Contains(out, `"namespace":"nl1"`) {
		t.Fatalf("expected fields for first identifier, got:\n%s", out)
	}
	if !strings.Contains(out, `"secret_id":"l2"`) || !strings.Contains(out, `"owner":"ol2"`) || !strings.Contains(out, `"namespace":"nl2"`) {
		t.Fatalf("expected fields for second identifier, got:\n%s", out)
	}
}

func TestParseVaultGatewayResponse_List_EmptySuccess(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	body := encodeRPCBodyFromPayload(buildListPayloadProtoEmptySuccess(t))
	if err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsList, body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Should log a single informational "no secrets found"
	if !strings.Contains(out, `"message":"no secrets found"`) {
		t.Fatalf("expected 'no secrets found' info log, got:\n%s", out)
	}
	// And no per-identifier lines
	if strings.Contains(out, `"message":"secret identifier"`) {
		t.Fatalf("did not expect 'secret identifier' logs, got:\n%s", out)
	}
}

func TestParseVaultGatewayResponse_List_Failure(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	body := encodeRPCBodyFromPayload(buildListPayloadProtoFailure(t, "boom"))
	err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsList, body)
	if err != nil {
		t.Fatalf("handler should not return error on list failure; it should log: %v", err)
	}

	out := buf.String()
	// One summary error line, no per-item logs
	if !strings.Contains(out, `"message":"list secrets failed"`) ||
		!strings.Contains(out, `"success":false`) ||
		!strings.Contains(out, `"error":"boom"`) {
		t.Fatalf("expected summary error log with error text, got:\n%s", out)
	}
	if strings.Contains(out, `"message":"secret identifier"`) {
		t.Fatalf("did not expect per-identifier logs on failure, got:\n%s", out)
	}
}

func TestParseVaultGatewayResponse_BadPayloadForList(t *testing.T) {
	var buf bytes.Buffer
	h := newTestHandler(&buf)

	// Wrong shape: identifiers should be an array
	raw := encodeRPCBodyFromPayload([]byte(`{"identifiers":"not-an-array"}`))
	err := h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsList, raw)
	if err == nil || !strings.Contains(err.Error(), "failed to decode list payload") {
		t.Fatalf("expected decode error for list payload, got: %v", err)
	}
}

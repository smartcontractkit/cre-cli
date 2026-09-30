package http_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	"github.com/smartcontractkit/chainlink/v2/core/utils"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/trigger"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

const (
	// Platform rows carry the workflow ID without a 0x prefix.
	platformWorkflowID = "00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856"
	wantWorkflowID     = "0x" + platformWorkflowID
)

type gatewayCall struct {
	method        string
	contentType   string
	accept        string
	authorization string
	body          []byte
}

type fakeGateway struct {
	*httptest.Server
	mu    sync.Mutex
	calls []gatewayCall
}

func (g *fakeGateway) recorded() []gatewayCall {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]gatewayCall(nil), g.calls...)
}

func newFakeGateway(t *testing.T, status int, reply func(requestID string) any) *fakeGateway {
	t.Helper()
	gw := &fakeGateway{}
	gw.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gw.mu.Lock()
		gw.calls = append(gw.calls, gatewayCall{
			method:        r.Method,
			contentType:   r.Header.Get("Content-Type"),
			accept:        r.Header.Get("Accept"),
			authorization: r.Header.Get("Authorization"),
			body:          body,
		})
		gw.mu.Unlock()

		var req jsonrpc2.Request[json.RawMessage]
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/jsonrpc")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(reply(req.ID))
	}))
	t.Cleanup(gw.Close)
	return gw
}

func acceptedReply(requestID string) any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  "workflows.execute",
		"result":  map[string]any{"workflow_id": platformWorkflowID, "workflow_execution_id": "0xe1", "status": "ACCEPTED"},
	}
}

func rpcErrorReply(code int64, message string) func(string) any {
	return func(requestID string) any {
		return map[string]any{
			"jsonrpc": "2.0",
			"id":      requestID,
			"error":   map[string]any{"code": code, "message": message},
		}
	}
}

func newKey(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	return key, hex.EncodeToString(crypto.FromECDSA(key))
}

func newRuntimeContext(gatewayURL, privateKeyHex string) *runtime.Context {
	logger := zerolog.Nop()
	s := &settings.Settings{
		User: settings.UserSettings{
			TargetName:  "staging",
			PrivateKeys: map[string]string{settings.EVM.Name: privateKeyHex},
		},
	}
	s.Workflow.UserWorkflowSettings.WorkflowName = "alpha"

	return &runtime.Context{
		Logger:         &logger,
		Settings:       s,
		Credentials:    &credentials.Credentials{AuthType: credentials.AuthTypeApiKey, APIKey: "test-key"},
		EnvironmentSet: &environments.EnvironmentSet{GatewayURL: gatewayURL},
		TenantContext:  &tenantctx.EnvironmentContext{},
	}
}

// execute runs `cre workflow trigger http` under a minimal root that mirrors the real global flags.
func execute(t *testing.T, rtCtx *runtime.Context, args ...string) (stdout string, err error) {
	t.Helper()
	root := &cobra.Command{Use: "cre", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("target", "", "")
	root.PersistentFlags().Bool("non-interactive", false, "")
	workflowCmd := &cobra.Command{Use: "workflow"}
	workflowCmd.AddCommand(trigger.New(rtCtx))
	root.AddCommand(workflowCmd)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs(append([]string{"workflow", "trigger", "http", "./my-workflow", "--target", "staging", "--non-interactive"}, args...))

	err = root.ExecuteContext(context.Background())
	return out.String(), err
}

// captureProcessStdout collects what ui helpers print straight to os.Stdout.
func captureProcessStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stdout
	os.Stdout = w
	fn()
	require.NoError(t, w.Close())
	os.Stdout = old
	var buf strings.Builder
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}

func decodeJSONOutput(t *testing.T, stdout string) map[string]json.RawMessage {
	t.Helper()
	var out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout must be a single JSON document, got: %q", stdout)
	return out
}

func TestTriggerHTTP_SignsAndPostsRequestAndPrintsJSON(t *testing.T) {
	gw := newFakeGateway(t, http.StatusOK, acceptedReply)
	key, keyHex := newKey(t)
	rtCtx := newRuntimeContext(gw.URL, keyHex)

	var stdout string
	processStdout := captureProcessStdout(t, func() {
		var err error
		// Upper-case, unprefixed ID checks that the CLI normalizes to what the Gateway accepts.
		stdout, err = execute(t, rtCtx, "--workflow-id", strings.ToUpper(platformWorkflowID), "--input", `{"orderId":42}`, "--json")
		require.NoError(t, err)
	})
	assert.Empty(t, processStdout, "with --json nothing but the JSON document may reach stdout")

	calls := gw.recorded()
	require.Len(t, calls, 1)
	call := calls[0]
	assert.Equal(t, http.MethodPost, call.method)
	assert.Equal(t, "application/jsonrpc", call.contentType)
	assert.Equal(t, "application/json", call.accept)
	token, ok := strings.CutPrefix(call.authorization, "Bearer ")
	require.True(t, ok, "Authorization must be a Bearer JWT, got %q", call.authorization)

	req, err := jsonrpc2.DecodeRequest[json.RawMessage](call.body, token)
	require.NoError(t, err)
	assert.Equal(t, "2.0", req.Version)
	assert.Equal(t, "workflows.execute", req.Method)
	_, parseErr := uuid.Parse(req.ID)
	require.NoError(t, parseErr)
	assert.JSONEq(t, `{"input":{"orderId":42},"workflow":{"workflowID":"`+wantWorkflowID+`"}}`, string(*req.Params))

	_, signer, err := utils.VerifyRequestJWT(token, req)
	require.NoError(t, err, "the Gateway must be able to verify the JWT against the body")
	assert.Equal(t, crypto.PubkeyToAddress(key.PublicKey), signer)

	out := decodeJSONOutput(t, stdout)
	assert.Len(t, out, 5, "unexpected keys in %s", stdout)
	assert.JSONEq(t, `"`+wantWorkflowID+`"`, string(out["workflowId"]))
	assert.JSONEq(t, `"`+req.ID+`"`, string(out["requestId"]))
	assert.JSONEq(t, `"`+gw.URL+`"`, string(out["gatewayUrl"]))
	assert.JSONEq(t, `"ACCEPTED"`, string(out["status"]))
	assert.JSONEq(t, `{"workflow_id":"`+platformWorkflowID+`","workflow_execution_id":"0xe1","status":"ACCEPTED"}`, string(out["result"]))
}

func TestTriggerHTTP_OutputJSONFlagMatchesJSONShorthand(t *testing.T) {
	gw := newFakeGateway(t, http.StatusOK, acceptedReply)
	_, keyHex := newKey(t)

	stdout, err := execute(t, newRuntimeContext(gw.URL, keyHex), "--workflow-id", wantWorkflowID, "--output", "json")

	require.NoError(t, err)
	assert.JSONEq(t, `"ACCEPTED"`, string(decodeJSONOutput(t, stdout)["status"]))
}

func TestTriggerHTTP_RejectsUnsupportedOutputFormat(t *testing.T) {
	gw := newFakeGateway(t, http.StatusOK, acceptedReply)
	_, keyHex := newKey(t)

	_, err := execute(t, newRuntimeContext(gw.URL, keyHex), "--workflow-id", wantWorkflowID, "--output", "yaml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "only \"json\" is accepted")
	assert.Empty(t, gw.recorded())
}

func TestTriggerHTTP_DefaultInputIsEmptyObject(t *testing.T) {
	gw := newFakeGateway(t, http.StatusOK, acceptedReply)
	_, keyHex := newKey(t)

	_, err := execute(t, newRuntimeContext(gw.URL, keyHex), "--workflow-id", wantWorkflowID, "--json")

	require.NoError(t, err)
	calls := gw.recorded()
	require.Len(t, calls, 1)
	var body struct {
		Params struct {
			Input json.RawMessage `json:"input"`
		} `json:"params"`
	}
	require.NoError(t, json.Unmarshal(calls[0].body, &body))
	assert.JSONEq(t, `{}`, string(body.Params.Input))
}

func TestTriggerHTTP_JSONRPCErrorPrintsErrorAndFails(t *testing.T) {
	gw := newFakeGateway(t, http.StatusBadRequest, rpcErrorReply(jsonrpc2.ErrInvalidRequest, "Auth failure: signer is not authorized"))
	_, keyHex := newKey(t)

	stdout, err := execute(t, newRuntimeContext(gw.URL, keyHex), "--workflow-id", wantWorkflowID, "--json")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Auth failure: signer is not authorized")

	out := decodeJSONOutput(t, stdout)
	assert.JSONEq(t, `"ERROR"`, string(out["status"]))
	assert.JSONEq(t, `null`, string(out["result"]))
	assert.JSONEq(t, `{"code":-32600,"message":"Auth failure: signer is not authorized"}`, string(out["error"]))
	assert.JSONEq(t, `"`+wantWorkflowID+`"`, string(out["workflowId"]))
	assert.NotEmpty(t, out["requestId"])
	assert.JSONEq(t, `"`+gw.URL+`"`, string(out["gatewayUrl"]))
}

func TestTriggerHTTP_JSONRPCErrorOnHTTP200StillFails(t *testing.T) {
	gw := newFakeGateway(t, http.StatusOK, rpcErrorReply(jsonrpc2.ErrInternal, "internal error"))
	_, keyHex := newKey(t)

	stdout, err := execute(t, newRuntimeContext(gw.URL, keyHex), "--workflow-id", wantWorkflowID, "--json")

	require.Error(t, err)
	assert.JSONEq(t, `{"code":-32603,"message":"internal error"}`, string(decodeJSONOutput(t, stdout)["error"]))
}

func TestTriggerHTTP_TransportFailureFailsWithJSONError(t *testing.T) {
	gw := newFakeGateway(t, http.StatusOK, acceptedReply)
	_, keyHex := newKey(t)
	rtCtx := newRuntimeContext(gw.URL, keyHex)
	gw.Close()

	stdout, err := execute(t, rtCtx, "--workflow-id", wantWorkflowID, "--json")

	require.Error(t, err)
	out := decodeJSONOutput(t, stdout)
	assert.JSONEq(t, `"ERROR"`, string(out["status"]))
	assert.Contains(t, string(out["error"]), "gateway")
}

func TestTriggerHTTP_ReadsInputFromFileRelativeToInvocationDir(t *testing.T) {
	gw := newFakeGateway(t, http.StatusOK, acceptedReply)
	_, keyHex := newKey(t)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "payload.json"), []byte("{\"from\": \"file\", \"n\": [1, 2]}\n"), 0o600))
	rtCtx := newRuntimeContext(gw.URL, keyHex)
	rtCtx.InvocationDir = dir

	_, err := execute(t, rtCtx, "--workflow-id", wantWorkflowID, "--input", "@payload.json", "--json")

	require.NoError(t, err)
	calls := gw.recorded()
	require.Len(t, calls, 1)
	var body struct {
		Params struct {
			Input json.RawMessage `json:"input"`
		} `json:"params"`
	}
	require.NoError(t, json.Unmarshal(calls[0].body, &body))
	assert.JSONEq(t, `{"from":"file","n":[1,2]}`, string(body.Params.Input))
}

func TestTriggerHTTP_InvalidInputFailsBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "malformed JSON", input: `{"orderId":`, wantErr: "--input is not valid JSON"},
		{name: "primitive", input: `42`, wantErr: "must be a JSON object or array"},
		{name: "missing file", input: "@does-not-exist.json", wantErr: "failed to read --input file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := newFakeGateway(t, http.StatusOK, acceptedReply)
			_, keyHex := newKey(t)

			stdout, err := execute(t, newRuntimeContext(gw.URL, keyHex), "--workflow-id", wantWorkflowID, "--input", tt.input, "--json")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
			assert.Empty(t, stdout)
			assert.Empty(t, gw.recorded())
		})
	}
}

func TestTriggerHTTP_RejectsMalformedWorkflowID(t *testing.T) {
	gw := newFakeGateway(t, http.StatusOK, acceptedReply)
	_, keyHex := newKey(t)

	_, err := execute(t, newRuntimeContext(gw.URL, keyHex), "--workflow-id", "0x1234")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--workflow-id")
	assert.Empty(t, gw.recorded())
}

func TestTriggerHTTP_RequiresWalletKey(t *testing.T) {
	for name, key := range map[string]string{
		"unset":       "",
		"placeholder": settings.DefaultEthPrivateKeyEnvPlaceholder,
	} {
		t.Run(name, func(t *testing.T) {
			gw := newFakeGateway(t, http.StatusOK, acceptedReply)

			_, err := execute(t, newRuntimeContext(gw.URL, key), "--workflow-id", wantWorkflowID)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "CRE_ETH_PRIVATE_KEY")
			assert.Empty(t, gw.recorded())
		})
	}
}

func TestTriggerHTTP_RejectsMalformedWalletKey(t *testing.T) {
	gw := newFakeGateway(t, http.StatusOK, acceptedReply)

	_, err := execute(t, newRuntimeContext(gw.URL, "not-a-key"), "--workflow-id", wantWorkflowID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid private key")
	assert.Empty(t, gw.recorded())
}

func TestTriggerHTTP_RejectsInsecureGatewayURL(t *testing.T) {
	_, keyHex := newKey(t)

	_, err := execute(t, newRuntimeContext("http://gateway.example.com", keyHex), "--workflow-id", wantWorkflowID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "https")
}

func TestTriggerHTTP_PrintsFriendlySummaryWithoutJSONFlag(t *testing.T) {
	gw := newFakeGateway(t, http.StatusOK, acceptedReply)
	_, keyHex := newKey(t)

	var cmdOut string
	uiOut := captureProcessStdout(t, func() {
		var err error
		cmdOut, err = execute(t, newRuntimeContext(gw.URL, keyHex), "--workflow-id", wantWorkflowID)
		require.NoError(t, err)
	})

	assert.Empty(t, cmdOut)
	assert.Contains(t, uiOut, "Trigger request sent")
	assert.Contains(t, uiOut, wantWorkflowID)
	assert.Contains(t, uiOut, gw.URL)
	assert.Contains(t, uiOut, "ACCEPTED")
}

func TestTriggerHTTP_ErrorWithoutJSONFlagPrintsNothingToStdout(t *testing.T) {
	gw := newFakeGateway(t, http.StatusBadRequest, rpcErrorReply(jsonrpc2.ErrInvalidRequest, "Workflow not found"))
	_, keyHex := newKey(t)

	var cmdOut string
	var err error
	uiOut := captureProcessStdout(t, func() {
		cmdOut, err = execute(t, newRuntimeContext(gw.URL, keyHex), "--workflow-id", wantWorkflowID)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Workflow not found")
	assert.Empty(t, cmdOut)
	assert.NotContains(t, uiOut, "Trigger request sent")
}

// platformServer serves GraphQL at /graphql and the Gateway at /gateway.
func platformServer(t *testing.T, workflows []map[string]any, onGateway http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		query, _ := body["query"].(string)
		require.Contains(t, query, "ListWorkflows")

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"workflows": map[string]any{"count": len(workflows), "data": workflows}},
		})
	})
	mux.HandleFunc("/gateway", onGateway)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func workflowRow(name, id, status string) map[string]any {
	return map[string]any{
		"uuid":           "uuid-" + id[:4] + status,
		"name":           name,
		"workflowId":     id,
		"ownerAddress":   "2020202020202020202020202020202020202020",
		"status":         status,
		"workflowSource": "private",
	}
}

func TestTriggerHTTP_ResolvesActiveWorkflowIDFromWorkflowName(t *testing.T) {
	const olderID = "9999999999999999999999999999999999999999999999999999999999999999"
	var gotID string
	srv := platformServer(t, []map[string]any{
		workflowRow("alpha", olderID, "DELETED"),
		workflowRow("alpha-suffix", olderID, "ACTIVE"),
		workflowRow("Alpha", platformWorkflowID, "ACTIVE"),
	}, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			ID     string `json:"id"`
			Params struct {
				Workflow struct {
					WorkflowID string `json:"workflowID"`
				} `json:"workflow"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		gotID = req.Params.Workflow.WorkflowID
		_ = json.NewEncoder(w).Encode(acceptedReply(req.ID))
	})
	_, keyHex := newKey(t)
	rtCtx := newRuntimeContext(srv.URL+"/gateway", keyHex)
	rtCtx.EnvironmentSet.GraphQLURL = srv.URL + "/graphql"

	stdout, err := execute(t, rtCtx, "--json")

	require.NoError(t, err)
	assert.Equal(t, wantWorkflowID, gotID)
	assert.JSONEq(t, `"`+wantWorkflowID+`"`, string(decodeJSONOutput(t, stdout)["workflowId"]))
	assert.Equal(t, platformWorkflowID, rtCtx.Workflow.ID, "telemetry expects the ID without a 0x prefix")
}

func TestTriggerHTTP_FailsWhenWorkflowNameHasNoMatch(t *testing.T) {
	gatewayHit := false
	srv := platformServer(t, []map[string]any{
		workflowRow("beta", platformWorkflowID, "ACTIVE"),
		workflowRow("alpha", platformWorkflowID, "DELETED"),
	}, func(http.ResponseWriter, *http.Request) { gatewayHit = true })
	_, keyHex := newKey(t)
	rtCtx := newRuntimeContext(srv.URL+"/gateway", keyHex)
	rtCtx.EnvironmentSet.GraphQLURL = srv.URL + "/graphql"

	_, err := execute(t, rtCtx, "--json")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `no workflow found with name "alpha"`)
	assert.False(t, gatewayHit)
}

func TestTriggerHTTP_FailsOnAmbiguousActiveWorkflows(t *testing.T) {
	srv := platformServer(t, []map[string]any{
		workflowRow("alpha", platformWorkflowID, "ACTIVE"),
		workflowRow("alpha", "7777777777777777777777777777777777777777777777777777777777777777", "ACTIVE"),
	}, func(http.ResponseWriter, *http.Request) { t.Error("gateway must not be called") })
	_, keyHex := newKey(t)
	rtCtx := newRuntimeContext(srv.URL+"/gateway", keyHex)
	rtCtx.EnvironmentSet.GraphQLURL = srv.URL + "/graphql"

	_, err := execute(t, rtCtx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple ACTIVE workflows")
	assert.Contains(t, err.Error(), "--workflow-id")
}

func TestTriggerHTTP_WorkflowIDFlagSkipsPlatformLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" {
			t.Error("platform must not be queried when --workflow-id is given")
		}
		body, _ := io.ReadAll(r.Body)
		var req jsonrpc2.Request[json.RawMessage]
		_ = json.Unmarshal(body, &req)
		_ = json.NewEncoder(w).Encode(acceptedReply(req.ID))
	}))
	t.Cleanup(srv.Close)
	_, keyHex := newKey(t)
	rtCtx := newRuntimeContext(srv.URL+"/gateway", keyHex)
	rtCtx.EnvironmentSet.GraphQLURL = srv.URL + "/graphql"

	_, err := execute(t, rtCtx, "--workflow-id", wantWorkflowID, "--json")

	require.NoError(t, err)
}

func TestTriggerHTTP_ResolutionRequiresUserContextAndWorkflowName(t *testing.T) {
	_, keyHex := newKey(t)
	gw := newFakeGateway(t, http.StatusOK, acceptedReply)

	t.Run("no user context", func(t *testing.T) {
		rtCtx := newRuntimeContext(gw.URL, keyHex)
		rtCtx.TenantContext = nil

		_, err := execute(t, rtCtx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "user context not available")
	})

	t.Run("no workflow name", func(t *testing.T) {
		rtCtx := newRuntimeContext(gw.URL, keyHex)
		rtCtx.Settings.Workflow.UserWorkflowSettings.WorkflowName = ""

		_, err := execute(t, rtCtx)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "workflow-name is not set")
	})

	assert.Empty(t, gw.recorded())
}

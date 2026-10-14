package list

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestNonInteractive_WithoutYes_ReturnsError(t *testing.T) {
	t.Parallel()
	v := viper.New()
	v.Set(settings.Flags.NonInteractive.Name, true)
	v.Set(settings.Flags.SkipConfirmation.Name, false)

	ctx := &runtime.Context{Viper: v}
	cmd := New(ctx)

	err := cmd.RunE(cmd, []string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required flags for --non-interactive mode")
}

func TestNonInteractive_WithYes_PassesGuard(t *testing.T) {
	t.Parallel()
	v := viper.New()
	v.Set(settings.Flags.NonInteractive.Name, true)
	v.Set(settings.Flags.SkipConfirmation.Name, true)

	ctx := &runtime.Context{Viper: v}
	cmd := New(ctx)

	err := cmd.RunE(cmd, []string{})
	// Guard passes; error comes from missing runtime setup, not the guard
	if err != nil {
		require.NotContains(t, err.Error(), "missing required flags for --non-interactive mode")
	}
}

func TestInvalidOutputFormat_ReturnsError(t *testing.T) {
	t.Parallel()
	v := viper.New()
	v.Set(settings.Flags.NonInteractive.Name, true)
	v.Set(settings.Flags.SkipConfirmation.Name, true)

	cmd := New(&runtime.Context{Viper: v})
	cmd.SetArgs([]string{"--output", "csv"})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "csv")
}

func TestOutputFlags_Registered(t *testing.T) {
	t.Parallel()
	cmd := New(&runtime.Context{Viper: viper.New()})
	require.NotNil(t, cmd.Flags().Lookup("output"))
	require.NotNil(t, cmd.Flags().Lookup("json"))
}

func TestPrintSecretsJSON(t *testing.T) {
	resp := &vault.ListSecretIdentifiersResponse{
		Success: true,
		Identifiers: []*vault.SecretIdentifier{
			{Key: "API_KEY", Owner: "0xabc", Namespace: "main"},
			nil,
			{Key: "DB_PASSWORD", Owner: "0xabc", Namespace: "main"},
		},
	}

	var err error
	out := captureStdout(t, func() { err = printSecretsJSON("0xabc", "main", resp) })
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got), "stdout must be only JSON, got %q", out)
	assert.Equal(t, map[string]any{
		"owner":     "0xabc",
		"namespace": "main",
		"secrets": []any{
			map[string]any{"id": "API_KEY", "namespace": "main", "owner": "0xabc"},
			map[string]any{"id": "DB_PASSWORD", "namespace": "main", "owner": "0xabc"},
		},
	}, got)
}

func TestPrintSecretsJSON_EmptyIsArray(t *testing.T) {
	var err error
	out := captureStdout(t, func() {
		err = printSecretsJSON("0xabc", "main", &vault.ListSecretIdentifiersResponse{Success: true})
	})
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, []any{}, got["secrets"])
}

func TestPrintSecretsJSON_FailureReturnsError(t *testing.T) {
	var err error
	out := captureStdout(t, func() {
		err = printSecretsJSON("0xabc", "main", &vault.ListSecretIdentifiersResponse{Success: false, Error: "boom"})
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
	assert.Empty(t, out)
}

func TestListResponseHandler_ReplacesDefaultOutput(t *testing.T) {
	payload, err := protojson.Marshal(&vault.ListSecretIdentifiersResponse{
		Success:     true,
		Identifiers: []*vault.SecretIdentifier{{Key: "API_KEY", Owner: "0xabc", Namespace: "main"}},
	})
	require.NoError(t, err)
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "req-1",
		"result":  map[string]any{"payload": json.RawMessage(payload)},
	})
	require.NoError(t, err)

	h := &common.Handler{}
	h.ListResponseHandler = func(resp *vault.ListSecretIdentifiersResponse) error {
		return printSecretsJSON("0xabc", "main", resp)
	}

	out := captureStdout(t, func() {
		err = h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsList, "req-1", body)
	})
	require.NoError(t, err)

	var got secretsListJSON
	require.NoError(t, json.Unmarshal([]byte(out), &got), "stdout must be only JSON, got %q", out)
	assert.Equal(t, secretsListJSON{
		Owner:     "0xabc",
		Namespace: "main",
		Secrets:   []secretIdentifierJSON{{ID: "API_KEY", Namespace: "main", Owner: "0xabc"}},
	}, got)
}

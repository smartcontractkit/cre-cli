package test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Reads deployed (runtime) bytecode from a Foundry artifact JSON.
func readDeployedBytecodeHex(t *testing.T, artifactPath string) string {
	t.Helper()

	var a struct {
		DeployedBytecode struct {
			Object string `json:"object"`
		} `json:"deployedBytecode"`
	}

	raw, err := os.ReadFile(artifactPath)
	require.NoError(t, err, "read forwarder artifact")

	require.NoError(t, json.Unmarshal(raw, &a), "unmarshal forwarder artifact")
	code := strings.TrimSpace(a.DeployedBytecode.Object)
	if !strings.HasPrefix(code, "0x") {
		code = "0x" + code
	}
	require.Greater(t, len(code), 2, "empty deployed bytecode in artifact")
	return code
}

// Calls Anvil's anvil_setCode to place the forwarder code at a given address.
func anvilSetCode(t *testing.T, rpcURL, addressHex, bytecodeHex string) {
	t.Helper()

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "anvil_setCode",
		"params":  []any{addressHex, bytecodeHex},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", rpcURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "anvil_setCode POST failed")
	defer resp.Body.Close()

	out, _ := io.ReadAll(resp.Body)
	var r struct {
		Result any              `json:"result"`
		Error  *json.RawMessage `json:"error"`
	}
	require.NoError(t, json.Unmarshal(out, &r), "decode anvil_setCode response")
	require.Nilf(t, r.Error, "anvil_setCode error: %s", string(out))
}

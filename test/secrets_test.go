package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common"
	"github.com/smartcontractkit/cre-cli/cmd/secrets/delete"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

// secretsCreateEoa writes a minimal secrets.json and invokes:
//
//	cre secrets create <secrets.json> <env-flag> <settings-flag>
//
// It returns whether an allowlist success log was observed and the output.
func (tc *TestConfig) secretsCreateEoa(t *testing.T) (bool, string) {
	t.Helper()

	// write secrets.json under the project root
	secretsPayload := []common.SecretItem{
		{ID: "testid", Value: "testval", Namespace: "main"},
	}
	b, err := json.MarshalIndent(secretsPayload, "", "  ")
	require.NoError(t, err, "marshal secrets.json")

	secretsPath := filepath.Join(tc.ProjectDirectory, "secrets.json")
	require.NoError(t, os.WriteFile(secretsPath, b, 0o600), "write secrets.json")

	// build CLI args
	args := []string{
		"secrets", "create",
		secretsPath,
		tc.GetCliEnvFlag(),
		tc.GetCliSettingsFlag(),
	}
	cmd := exec.Command(CLIPath, args...)
	cmd.Dir = tc.ProjectDirectory

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()
	out := stdout.String() + stderr.String()

	allowed := strings.Contains(out, "Digest allowlisted; proceeding to gateway POST") ||
		strings.Contains(out, "Digest already allowlisted; skipping on-chain allowlist")

	return allowed, stripANSI(out)
}

// secretsUpdateEoa writes an updated secrets.json and invokes:
//
//	cre secrets update <secrets.json> <env-flag> <settings-flag>
//
// It returns whether an allowlist success log was observed and the output.
func (tc *TestConfig) secretsUpdateEoa(t *testing.T) (bool, string) {
	t.Helper()

	// re-write secrets.json with updated value (same id/namespace)
	secretsPayload := []common.SecretItem{
		{ID: "testid", Value: "updated-val", Namespace: "testns"},
	}
	b, err := json.MarshalIndent(secretsPayload, "", "  ")
	require.NoError(t, err, "marshal secrets.json")

	secretsPath := filepath.Join(tc.ProjectDirectory, "secrets.json")
	require.NoError(t, os.WriteFile(secretsPath, b, 0o600), "write secrets.json")

	args := []string{
		"secrets", "update",
		secretsPath,
		tc.GetCliEnvFlag(),
		tc.GetCliSettingsFlag(),
	}
	cmd := exec.Command(CLIPath, args...)
	cmd.Dir = tc.ProjectDirectory

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()
	out := stdout.String() + stderr.String()

	allowed := strings.Contains(out, "Digest allowlisted; proceeding to gateway POST") ||
		strings.Contains(out, "Digest already allowlisted; skipping on-chain allowlist")

	return allowed, stripANSI(out)
}

// secretsListEoa invokes:
//
//	cre secrets list --namespace <ns> <env-flag> <settings-flag>
func (tc *TestConfig) secretsListEoa(t *testing.T, ns string) (bool, string) {
	t.Helper()

	args := []string{
		"secrets", "list",
		"--namespace", ns,
		tc.GetCliEnvFlag(),
		tc.GetCliSettingsFlag(),
	}
	cmd := exec.Command(CLIPath, args...)
	cmd.Dir = tc.ProjectDirectory

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()
	out := stdout.String() + stderr.String()

	allowed := strings.Contains(out, "Digest allowlisted; proceeding to gateway POST") ||
		strings.Contains(out, "Digest already allowlisted; skipping on-chain allowlist")

	return allowed, stripANSI(out)
}

// secretsDeleteEoa writes a minimal delete file and invokes:
//
//	cre secrets delete <file> <env-flag> <settings-flag>
func (tc *TestConfig) secretsDeleteEoa(t *testing.T, ns string) (bool, string) {
	t.Helper()

	payload := []delete.DeleteSecretItem{
		{ID: "testid", Namespace: ns},
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err, "marshal delete secrets json")

	delPath := filepath.Join(tc.ProjectDirectory, "secrets-delete.json")
	require.NoError(t, os.WriteFile(delPath, b, 0o600), "write secrets-delete.json")

	args := []string{
		"secrets", "delete",
		delPath,
		tc.GetCliEnvFlag(),
		tc.GetCliSettingsFlag(),
	}
	cmd := exec.Command(CLIPath, args...)
	cmd.Dir = tc.ProjectDirectory

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	_ = cmd.Run()

	out := stdout.String() + stderr.String()
	allowed := strings.Contains(out, "Digest allowlisted; proceeding to gateway POST") ||
		strings.Contains(out, "Digest already allowlisted; skipping on-chain allowlist")
	return allowed, stripANSI(out)
}

// create -> update -> list -> delete
func TestCLISecretsWithEoa(t *testing.T) {
	// start anvil backend, load state with pre-deployed contracts
	anvilProc, testEthUrl := initTestEnv(t)
	defer StopAnvil(anvilProc)

	tc := NewTestConfig(t)

	// create env file. use address3 which is linked
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey3), "Failed to create env file")

	// create workflow.yaml
	require.NoError(t, createCliSettingsFile(tc, constants.TestAddress3, "myworkflow", testEthUrl), "Failed to create cre config file")

	// create blank project.yaml
	require.NoError(t, createBlankProjectSettingFile(), "Failed to create project.yaml setting file")

	t.Cleanup(tc.Cleanup(t))

	// the following contracts are already deployed and configured in the Anvil state file
	t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
	t.Setenv(environments.EnvVarWorkflowRegistryChainSelector, strconv.FormatUint(TestChainSelector, 10))
	t.Setenv(environments.EnvVarCapabilitiesRegistryAddress, "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9")
	t.Setenv(environments.EnvVarCapabilitiesRegistryChainSelector, strconv.FormatUint(TestChainSelector, 10))

	// set up a mock server to simulate the vault gateway
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type reqEnvelope struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      any             `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		var req reqEnvelope
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Build the proto payload as JSON
		var payloadJSON []byte
		switch req.Method {
		case vaulttypes.MethodSecretsCreate:
			respProto := &vault.CreateSecretsResponse{
				Responses: []*vault.CreateSecretResponse{
					{
						Id:      &vault.SecretIdentifier{Key: "testid", Namespace: "main", Owner: constants.TestAddress3},
						Success: true,
					},
				},
			}
			payloadJSON, _ = protojson.Marshal(respProto)

		case vaulttypes.MethodSecretsUpdate:
			respProto := &vault.UpdateSecretsResponse{
				Responses: []*vault.UpdateSecretResponse{
					{
						Id:      &vault.SecretIdentifier{Key: "testid", Namespace: "main", Owner: constants.TestAddress3},
						Success: true,
					},
				},
			}
			payloadJSON, _ = protojson.Marshal(respProto)
		case vaulttypes.MethodSecretsList:
			respProto := &vault.ListSecretIdentifiersResponse{
				Identifiers: []*vault.SecretIdentifier{
					{Key: "testid", Namespace: "main", Owner: constants.TestAddress3},
				},
				Success: true,
			}
			payloadJSON, _ = protojson.Marshal(respProto)
		case vaulttypes.MethodSecretsDelete:
			respProto := &vault.DeleteSecretsResponse{
				Responses: []*vault.DeleteSecretResponse{
					{
						Id:      &vault.SecretIdentifier{Key: "testid", Namespace: "main", Owner: constants.TestAddress3},
						Success: true,
					},
				},
			}
			payloadJSON, _ = protojson.Marshal(respProto)
		default:
			t.Fatal("unexpected method", req.Method)
			return
		}

		// use json.RawMessage for "payload" so it's embedded as raw JSON.
		type result struct {
			Payload json.RawMessage `json:"payload"`
		}
		type respEnvelope struct {
			JSONRPC string `json:"jsonrpc"`
			ID      any    `json:"id"`
			Result  result `json:"result,omitempty"`
			Error   any    `json:"error,omitempty"`
		}
		out := respEnvelope{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result{Payload: json.RawMessage(payloadJSON)},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()

	// set the above mocked server as Gateway endpoint
	t.Setenv(environments.EnvVarVaultGatewayURL, srv.URL)

	// --- CREATE ---
	allowedCreate, outCreate := tc.secretsCreateEoa(t)
	if !allowedCreate {
		t.Fatalf("allowlist not detected for create.\n\nCLI OUTPUT:\n%s", outCreate)
	}
	require.Contains(t, outCreate, "secret created", "expected create log.\nCLI OUTPUT:\n%s", outCreate)
	require.Contains(t, outCreate, "success=true", "create should not fail.\nCLI OUTPUT:\n%s", outCreate)

	// --- UPDATE ---
	allowedUpdate, outUpdate := tc.secretsUpdateEoa(t)
	if !allowedUpdate {
		t.Fatalf("allowlist not detected for update.\n\nCLI OUTPUT:\n%s", outUpdate)
	}
	require.Contains(t, outUpdate, "secret updated", "expected update log.\nCLI OUTPUT:\n%s", outUpdate)
	require.Contains(t, outUpdate, "success=true", "update should not fail.\nCLI OUTPUT:\n%s", outUpdate)

	// --- LIST ---
	allowedList, outList := tc.secretsListEoa(t, "main")
	if !allowedList {
		t.Fatalf("allowlist not detected for list.\n\nCLI OUTPUT:\n%s", outList)
	}
	require.Contains(t, outList, "testid", "expected listed secret id in output.\nCLI OUTPUT:\n%s", outList)
	require.Contains(t, outList, "namespace=main", "expected namespace in list output.\nCLI OUTPUT:\n%s", outList)
	require.Contains(t, outList, "success=true", "list should not fail.\nCLI OUTPUT:\n%s", outList)

	// --- DELETE ---
	allowedDelete, outDelete := tc.secretsDeleteEoa(t, "main")
	if !allowedDelete {
		t.Fatalf("allowlist not detected for delete.\n\nCLI OUTPUT:\n%s", outDelete)
	}
	require.Contains(t, outDelete, "testid", "expected listed secret id in output.\nCLI OUTPUT:\n%s", outDelete)
	require.Contains(t, outDelete, "namespace=main", "expected namespace in delete output.\nCLI OUTPUT:\n%s", outDelete)
	require.Contains(t, outDelete, "success=true", "delete should not fail.\nCLI OUTPUT:\n%s", outDelete)

}

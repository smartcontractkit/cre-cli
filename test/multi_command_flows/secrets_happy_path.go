package multi_command_flows

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common"
	"github.com/smartcontractkit/cre-cli/cmd/secrets/delete"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

// getProjectDirectory extracts the project directory from the project root flag
func getProjectDirectory(tc TestConfig) string {
	flag := tc.GetProjectRootFlag()
	// Format is "--project-root=/path/to/directory"
	parts := strings.Split(flag, "=")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// RunSecretsHappyPath runs the complete secrets happy path workflow:
// Create -> Update -> List -> Delete
func RunSecretsHappyPath(t *testing.T, tc TestConfig, chainName string) {
	t.Helper()

	// Set up environment variables for pre-deployed contracts
	t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
	t.Setenv(environments.EnvVarWorkflowRegistryChainName, chainName)
	t.Setenv(environments.EnvVarCapabilitiesRegistryAddress, "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9")
	t.Setenv(environments.EnvVarCapabilitiesRegistryChainName, chainName)

	// Set up a mock server to simulate the vault gateway
	// Set dummy API key
	t.Setenv(credentials.CreApiKeyVar, "test-api")

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
						Id:      &vault.SecretIdentifier{Key: "testid", Namespace: "testns", Owner: constants.TestAddress3},
						Success: true,
					},
				},
			}
			payloadJSON, _ = protojson.Marshal(respProto)
		case vaulttypes.MethodSecretsList:
			respProto := &vault.ListSecretIdentifiersResponse{
				Identifiers: []*vault.SecretIdentifier{
					{Key: "testid", Namespace: "testns", Owner: constants.TestAddress3},
				},
				Success: true,
			}
			payloadJSON, _ = protojson.Marshal(respProto)
		case vaulttypes.MethodSecretsDelete:
			respProto := &vault.DeleteSecretsResponse{
				Responses: []*vault.DeleteSecretResponse{
					{
						Id:      &vault.SecretIdentifier{Key: "testid", Namespace: "testns", Owner: constants.TestAddress3},
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

	// Set the above mocked server as Gateway endpoint
	t.Setenv(environments.EnvVarVaultGatewayURL, srv.URL)

	// ===== PHASE 1: CREATE SECRETS =====
	t.Run("Create", func(t *testing.T) {
		allowed, out := secretsCreateEoa(t, tc)
		if !allowed {
			t.Fatalf("allowlist not detected for create.\n\nCLI OUTPUT:\n%s", out)
		}
		require.Contains(t, out, "secret created", "expected create log.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "success=true", "create should not fail.\nCLI OUTPUT:\n%s", out)
	})

	// ===== PHASE 2: UPDATE SECRETS =====
	t.Run("Update", func(t *testing.T) {
		allowed, out := secretsUpdateEoa(t, tc)
		if !allowed {
			t.Fatalf("allowlist not detected for update.\n\nCLI OUTPUT:\n%s", out)
		}
		require.Contains(t, out, "secret updated", "expected update log.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "success=true", "update should not fail.\nCLI OUTPUT:\n%s", out)
	})

	// ===== PHASE 3: LIST SECRETS =====
	t.Run("List", func(t *testing.T) {
		allowed, out := secretsListEoa(t, tc, "testns")
		if !allowed {
			t.Fatalf("allowlist not detected for list.\n\nCLI OUTPUT:\n%s", out)
		}
		require.Contains(t, out, "testid", "expected listed secret id in output.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "namespace=testns", "expected namespace in list output.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "success=true", "list should not fail.\nCLI OUTPUT:\n%s", out)
	})

	// ===== PHASE 4: DELETE SECRETS =====
	t.Run("Delete", func(t *testing.T) {
		allowed, out := secretsDeleteEoa(t, tc, "testns")
		if !allowed {
			t.Fatalf("allowlist not detected for delete.\n\nCLI OUTPUT:\n%s", out)
		}
		require.Contains(t, out, "testid", "expected listed secret id in output.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "namespace=testns", "expected namespace in delete output.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "success=true", "delete should not fail.\nCLI OUTPUT:\n%s", out)
	})
}

// secretsCreateEoa writes a minimal secrets.json and invokes:
//
//	cre secrets create <secrets.json> <env-flag> <settings-flag>
//
// It returns whether an allowlist success log was observed and the output.
func secretsCreateEoa(t *testing.T, tc TestConfig) (bool, string) {
	t.Helper()

	// write secrets.json under the project root
	secretsPayload := []common.SecretItem{
		{ID: "testid", Value: "testval", Namespace: "main"},
	}
	b, err := json.MarshalIndent(secretsPayload, "", "  ")
	require.NoError(t, err, "marshal secrets.json")

	projectDir := getProjectDirectory(tc)
	secretsPath := filepath.Join(projectDir, "secrets.json")
	require.NoError(t, os.WriteFile(secretsPath, b, 0o600), "write secrets.json")

	// build CLI args
	args := []string{
		"secrets", "create",
		secretsPath,
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
	}
	cmd := exec.Command(CLIPath, args...)
	// Let CLI handle context switching - don't set cmd.Dir manually

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()
	out := stdout.String() + stderr.String()

	allowed := strings.Contains(out, "Digest allowlisted; proceeding to gateway POST") ||
		strings.Contains(out, "Digest already allowlisted; skipping on-chain allowlist")

	return allowed, StripANSI(out)
}

// secretsUpdateEoa writes an updated secrets.json and invokes:
//
//	cre secrets update <secrets.json> <env-flag> <settings-flag>
//
// It returns whether an allowlist success log was observed and the output.
func secretsUpdateEoa(t *testing.T, tc TestConfig) (bool, string) {
	t.Helper()

	// re-write secrets.json with updated value (same id, different namespace)
	secretsPayload := []common.SecretItem{
		{ID: "testid", Value: "updated-val", Namespace: "testns"},
	}
	b, err := json.MarshalIndent(secretsPayload, "", "  ")
	require.NoError(t, err, "marshal secrets.json")

	projectDir := getProjectDirectory(tc)
	secretsPath := filepath.Join(projectDir, "secrets.json")
	require.NoError(t, os.WriteFile(secretsPath, b, 0o600), "write secrets.json")

	args := []string{
		"secrets", "update",
		secretsPath,
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
	}
	cmd := exec.Command(CLIPath, args...)
	// Let CLI handle context switching - don't set cmd.Dir manually

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()
	out := stdout.String() + stderr.String()

	allowed := strings.Contains(out, "Digest allowlisted; proceeding to gateway POST") ||
		strings.Contains(out, "Digest already allowlisted; skipping on-chain allowlist")

	return allowed, StripANSI(out)
}

// secretsListEoa invokes:
//
//	cre secrets list --namespace <ns> <env-flag> <settings-flag>
func secretsListEoa(t *testing.T, tc TestConfig, ns string) (bool, string) {
	t.Helper()

	args := []string{
		"secrets", "list",
		"--namespace", ns,
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
	}
	cmd := exec.Command(CLIPath, args...)
	// Let CLI handle context switching - don't set cmd.Dir manually

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()
	out := stdout.String() + stderr.String()

	allowed := strings.Contains(out, "Digest allowlisted; proceeding to gateway POST") ||
		strings.Contains(out, "Digest already allowlisted; skipping on-chain allowlist")

	return allowed, StripANSI(out)
}

// secretsDeleteEoa writes a minimal delete file and invokes:
//
//	cre secrets delete <file> <env-flag> <settings-flag>
func secretsDeleteEoa(t *testing.T, tc TestConfig, ns string) (bool, string) {
	t.Helper()

	payload := []delete.DeleteSecretItem{
		{ID: "testid", Namespace: ns},
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err, "marshal delete secrets json")

	projectDir := getProjectDirectory(tc)
	delPath := filepath.Join(projectDir, "secrets-delete.json")
	require.NoError(t, os.WriteFile(delPath, b, 0o600), "write secrets-delete.json")

	args := []string{
		"secrets", "delete",
		delPath,
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
	}
	cmd := exec.Command(CLIPath, args...)
	// Let CLI handle context switching - don't set cmd.Dir manually

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	_ = cmd.Run()

	out := stdout.String() + stderr.String()
	allowed := strings.Contains(out, "Digest allowlisted; proceeding to gateway POST") ||
		strings.Contains(out, "Digest already allowlisted; skipping on-chain allowlist")
	return allowed, StripANSI(out)
}

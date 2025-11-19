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
	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common"
	"github.com/smartcontractkit/cre-cli/cmd/secrets/delete"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

// Hex-encoded tdh2easy.PublicKey blob returned by the gateway
const vaultPublicKeyHex = "7b2247726f7570223a2250323536222c22475f626172223a22424d704759487a2b33333432596436582f2b6d4971396d5468556c6d2f317355716b51783333343564303373472b2f2f307257494d39795a70454b44566c6c2b616f36586c513743366546452b665472356568785a4f343d222c2248223a22424257546f7638394b546b41505a7566474454504e35626f456d6453305368697975696e3847336e58517774454931536333394453314b41306a595a6576546155476775444d694431746e6e4d686575373177574b57593d222c22484172726179223a5b22424937726649364c646f7654413948676a684b5955516a4744456a5a66374f30774378466c432f2f384e394733464c796247436d6e54734236632b50324c34596a39477548555a4936386d54342b4e77786f794b6261513d222c22424736634369395574317a65433753786b4c442b6247354751505473717463324a7a544b4c726b784d496e4c36484e7658376541324b6167423243447a4b6a6f76783570414c6a74523734537a6c7146543366746662513d222c224245576f7147546d6b47314c31565a53655874345147446a684d4d2b656e7a6b426b7842782b484f72386e39336b51543963594938486f513630356a65504a732f53575866355a714534564e676b4f672f643530395a6b3d222c22424a31552b6e5344783269567a654177475948624e715242564869626b74466b624f4762376158562f3946744c6876314b4250416c3272696e73714171754459504e2f54667870725a6e655259594a2b2f453162536a673d222c224243675a623770424d777732337138577767736e322b6c4d665259343561347576445345715a7559614e2f356e64744970355a492f4a6f454d372b36304a6338735978682b535365364645683052364f57666855706d453d222c2242465a5942524a336d6647695644312b4f4b4e4f374c54355a6f6574515442624a6b464152757143743268492f52757832756b7166794c6c364d71566e55613557336e49726e71506132566d5345755758546d39456f733d222c22424f716b662f356232636c4d314a78615831446d6a76494c4437334f6734566b42732f4b686b6e4d6867435772552f30574a36734e514a6b425462686b4a5535576b48506342626d45786c6362706a49743349494632303d225d7d"

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

	// Set up a mock server to simulate the vault gateway
	// Set dummy API key
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	// set up a mock server to simulate the vault gateway
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/graphql" {
			var req graphQLRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")

			// Handle authentication validation query
			if strings.Contains(req.Query, "getOrganization") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getOrganization": map[string]any{
							"organizationId": "test-org-id",
						},
					},
				})
				return
			}

			// Handle listWorkflowOwners query
			if strings.Contains(req.Query, "listWorkflowOwners") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"listWorkflowOwners": map[string]any{
							"linkedOwners": []map[string]string{
								{
									"workflowOwnerAddress": strings.ToLower(constants.TestAddress3), // linked owner
									"verificationStatus":   "VERIFICATION_STATUS_SUCCESSFULL",       //nolint:misspell // Intentional misspelling to match external API
								},
							},
						},
					},
				})
				return
			}

			// Fallback error
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
			})
			return
		}

		type reqEnvelope struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      any             `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		var req reqEnvelope
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Handle the new public key fetch.
		if req.Method == vaulttypes.MethodPublicKeyGet {
			type pkResult struct {
				PublicKey string `json:"publicKey"`
			}
			out := map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"method":  vaulttypes.MethodPublicKeyGet,
				"result":  pkResult{PublicKey: vaultPublicKeyHex},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(out)
			return
		}

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
	t.Setenv(environments.EnvVarGraphQLURL, srv.URL+"/graphql")

	// ===== PHASE 1: CREATE SECRETS =====
	t.Run("Create", func(t *testing.T) {
		allowed, out := secretsCreateEoa(t, tc)
		if !allowed {
			t.Fatalf("allowlist not detected for create.\n\nCLI OUTPUT:\n%s", out)
		}
		require.Contains(t, out, "Secret created:", "expected create log.\nCLI OUTPUT:\n%s", out)
	})

	// ===== PHASE 2: UPDATE SECRETS =====
	t.Run("Update", func(t *testing.T) {
		allowed, out := secretsUpdateEoa(t, tc)
		if !allowed {
			t.Fatalf("allowlist not detected for update.\n\nCLI OUTPUT:\n%s", out)
		}
		require.Contains(t, out, "Secret updated:", "expected update log.\nCLI OUTPUT:\n%s", out)
	})

	// ===== PHASE 3: LIST SECRETS =====
	t.Run("List", func(t *testing.T) {
		allowed, out := secretsListEoa(t, tc, "testns")
		if !allowed {
			t.Fatalf("allowlist not detected for list.\n\nCLI OUTPUT:\n%s", out)
		}
		require.Contains(t, out, "testid", "expected listed secret id in output.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "namespace=testns", "expected namespace in list output.\nCLI OUTPUT:\n%s", out)
	})

	// ===== PHASE 4: DELETE SECRETS =====
	t.Run("Delete", func(t *testing.T) {
		allowed, out := secretsDeleteEoa(t, tc, "testns")
		if !allowed {
			t.Fatalf("allowlist not detected for delete.\n\nCLI OUTPUT:\n%s", out)
		}
		require.Contains(t, out, "Secret deleted:", "expected delete log.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "testid", "expected listed secret id in output.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "namespace=testns", "expected namespace in delete output.\nCLI OUTPUT:\n%s", out)
	})
}

// RunSecretsListMsig on unsigned
func RunSecretsListMsig(t *testing.T, tc TestConfig, chainName string) {
	t.Helper()

	// Set up environment variables for pre-deployed contracts
	t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
	t.Setenv(environments.EnvVarWorkflowRegistryChainName, chainName)

	// Set up a mock server to simulate the vault gateway
	// Set dummy API key
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	gqlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost:
			var req graphQLRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")

			// Handle authentication validation query
			if strings.Contains(req.Query, "getOrganization") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"getOrganization": map[string]any{
							"organizationId": "test-org-id",
						},
					},
				})
				return
			}

			if strings.Contains(req.Query, "listWorkflowOwners") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"listWorkflowOwners": map[string]any{
							"linkedOwners": []map[string]string{
								{
									"workflowOwnerAddress": constants.TestAddress3,
									"verificationStatus":   "VERIFICATION_STATUS_SUCCESSFULL", //nolint:misspell
								},
							},
						},
					},
				})
				return
			}

			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
			})
			return

		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
			return
		}
	}))
	defer gqlSrv.Close()

	// Point GraphQL client to mock (no gateway needed for unsigned list)
	t.Setenv(environments.EnvVarGraphQLURL, gqlSrv.URL+"/graphql")

	t.Run("ListMsig", func(t *testing.T) {
		out := secretsListMsig(t, tc)
		require.Contains(t, out, "MSIG transaction prepared", "expected transaction prepared.\nCLI OUTPUT:\n%s", out)
	})
}

//	cre secrets list <env-flag> <settings-flag> --unsigned
//
// It returns the output.
func secretsListMsig(t *testing.T, tc TestConfig) string {
	t.Helper()

	// build CLI args
	args := []string{
		"secrets", "list",
		tc.GetCliEnvFlag(),
		tc.GetProjectRootFlag(),
		"--unsigned",
	}
	cmd := exec.Command(CLIPath, args...)
	// Let CLI handle context switching - don't set cmd.Dir manually

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run()
	out := stdout.String() + stderr.String()

	return StripANSI(out)
}

// secretsCreateEoa writes a minimal secrets.yaml and invokes:
//
//	cre secrets create <secrets.yaml> <env-flag> <settings-flag>
//
// It returns whether an allowlist success log was observed and the output.
func secretsCreateEoa(t *testing.T, tc TestConfig) (bool, string) {
	t.Helper()

	// write secrets.yaml under the project root
	cfg := common.SecretsYamlConfig{
		SecretsNames: map[string][]string{
			"testid": {"TESTID_ENV"},
		},
	}

	b, err := yaml.Marshal(cfg)
	require.NoError(t, err, "marshal secrets.yaml")

	projectDir := getProjectDirectory(tc)
	secretsPath := filepath.Join(projectDir, "secrets.yaml")
	require.NoError(t, os.WriteFile(secretsPath, b, 0o600), "write secrets.yaml")

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

// secretsUpdateEoa writes an updated secrets.yaml and invokes:
//
//	cre secrets update <secrets.yaml> <env-flag> <settings-flag>
//
// It returns whether an allowlist success log was observed and the output.
func secretsUpdateEoa(t *testing.T, tc TestConfig) (bool, string) {
	t.Helper()

	// re-write secrets.json with updated value (same id, different namespace)
	// write secrets.yaml under the project root
	cfg := common.SecretsYamlConfig{
		SecretsNames: map[string][]string{
			"testid": {"TESTID_ENV_UPDATED"},
		},
	}

	b, err := yaml.Marshal(cfg)
	require.NoError(t, err, "marshal secrets.yaml")

	projectDir := getProjectDirectory(tc)
	secretsPath := filepath.Join(projectDir, "secrets.yaml")
	require.NoError(t, os.WriteFile(secretsPath, b, 0o600), "write secrets.yaml")

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

	cfg := delete.SecretsDeleteYamlConfig{
		SecretsNames: []string{"testid"},
	}

	b, err := yaml.Marshal(cfg)
	require.NoError(t, err, "marshal delete secrets yaml")

	projectDir := getProjectDirectory(tc)
	delPath := filepath.Join(projectDir, "secrets-delete.yaml")
	require.NoError(t, os.WriteFile(delPath, b, 0o600), "write secrets-delete.yaml")

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

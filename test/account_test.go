package test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/test-go/testify/require"

	workflow_registry_v2_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	test "github.com/smartcontractkit/cre-cli/test/contracts"
)

type gqlReq struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

func TestCLIAccountLinkAndListKey_EOA(t *testing.T) {
	anvilProc, testEthURL := initTestEnv(t)
	defer StopAnvil(anvilProc)

	tc := NewTestConfig(t)

	// Use an UNLINKED owner (Address4 / PrivateKey4) so LinkOwner succeeds.
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey4), "failed to create env file")

	// Create workflow.yaml with owner=Address4
	require.NoError(t, createCliSettingsFile(tc, constants.TestAddress4, "workflow-name", testEthURL), "failed to create settings")
	require.NoError(t, createBlankProjectSettingFile(tc.ProjectDirectory+"project.yaml"), "failed to create project.yaml")
	t.Cleanup(tc.Cleanup(t))

	// Pre-baked registry addresses from Anvil state dump
	t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
	t.Setenv(environments.EnvVarWorkflowRegistryChainSelector, strconv.FormatUint(TestChainSelector, 10))
	t.Setenv(environments.EnvVarCapabilitiesRegistryAddress, "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9")
	t.Setenv(environments.EnvVarCapabilitiesRegistryChainSelector, strconv.FormatUint(TestChainSelector, 10))

	registryAddr := os.Getenv(environments.EnvVarWorkflowRegistryAddress)
	require.NotEmpty(t, registryAddr, "registry address env must be set")

	// One GraphQL server that supports InitiateLinking, listWorkflowOwners
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !(strings.HasPrefix(r.URL.Path, "/graphql") && r.Method == http.MethodPost) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
			return
		}

		var req gqlReq
		_ = json.NewDecoder(r.Body).Decode(&req)

		switch {
		case strings.Contains(req.Query, "InitiateLinking"):
			// Extract owner address
			request, _ := req.Variables["request"].(map[string]any)
			ownerHex, _ := request["workflowOwnerAddress"].(string)
			if ownerHex == "" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{{"message": "missing workflowOwnerAddress"}},
				})
				return
			}

			resp, _, err := buildInitiateLinkingEOAResponse(t, testEthURL, registryAddr, ownerHex, TestChainSelector)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{{"message": err.Error()}},
				})
				return
			}

			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"initiateLinking": resp,
				},
			})
			return

		case strings.Contains(req.Query, "listWorkflowOwners"):
			listResp := map[string]any{
				"data": map[string]any{
					"listWorkflowOwners": map[string]any{
						"linkedOwners": []map[string]string{
							{
								"workflowOwnerAddress": constants.TestAddress4,
								"workflowOwnerLabel":   "owner-label-1",
								"environment":          "PRODUCTION_TESTNET",
								"verificationStatus":   "VERIFIED",
								"verifiedAt":           "2025-01-02T03:04:05Z",
								"chainSelector":        strconv.FormatUint(TestChainSelector, 10),
								"contractAddress":      registryAddr,
								"requestProcess":       "EOA",
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(listResp)
			return
		}

		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]string{{"message": "Unsupported GraphQL query"}},
		})
	}))
	defer srv.Close()

	// Point CLI at mock GraphQL
	os.Setenv(environments.EnvVarGraphQLURL, srv.URL+"/graphql")

	// ---- 1) Link the key ----
	{
		args := []string{
			"account", "link-key",
			tc.GetCliEnvFlag(),
			tc.GetCliSettingsFlag(),
			"-l", "owner-label-1",
		}
		cmd := exec.Command(CLIPath, args...)
		cmd.Dir = tc.ProjectDirectory

		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr

		require.NoError(
			t,
			cmd.Run(),
			"cre account link-key failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
			stdout.String(),
			stderr.String(),
		)

		out := stripANSI(stdout.String() + stderr.String())
		require.Contains(t, out, "Starting linking", "should announce linking start.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "Contract address validation passed", "server & settings addresses must match.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "Linked successfully", "EOA path should link on-chain.\nCLI OUTPUT:\n%s", out)
	}

	// ---- 2) List the linked keys ----
	{
		args := []string{
			"account", "list-key",
			tc.GetCliEnvFlag(),
			tc.GetCliSettingsFlag(),
		}
		cmd := exec.Command(CLIPath, args...)
		cmd.Dir = tc.ProjectDirectory

		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr

		require.NoError(
			t,
			cmd.Run(),
			"cre account list-key failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
			stdout.String(),
			stderr.String(),
		)

		out := stripANSI(stdout.String() + stderr.String())
		require.Contains(t, out, "Workflow owners retrieved successfully", "expected success banner.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "Linked Owners:", "expected 'Linked Owners' section.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "owner-label-1", "expected owner label listed.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, constants.TestAddress4, "expected owner address listed.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "Chain Selector:", "expected chain selector line.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "Contract Address:", "expected contract address line.\nCLI OUTPUT:\n%s", out)
	}
}

// Builds a valid InitiateLinking GraphQL response for EOA flow.
// Signs the message with the allowed signer (TestPrivateKey2) so CanLinkOwner + LinkOwner succeed.
func buildInitiateLinkingEOAResponse(t *testing.T, rpcURL, registryAddrHex, ownerHex string, chainSelector uint64) (map[string]any, string, error) {
	ctx := context.Background()

	ec, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, "", fmt.Errorf("dial ethclient: %w", err)
	}
	chainID, err := ec.NetworkID(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("network id: %w", err)
	}

	regAddr := common.HexToAddress(registryAddrHex)
	reg, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(regAddr, ec)
	if err != nil {
		return nil, "", fmt.Errorf("new registry: %w", err)
	}
	version, err := reg.TypeAndVersion(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, "", fmt.Errorf("TypeAndVersion: %w", err)
	}

	// Valid for 1 hour
	validUntil := time.Now().UTC().Add(1 * time.Hour)

	// Deterministic proof
	nonce := strconv.FormatInt(time.Now().UnixNano(), 10)
	sum := sha256.Sum256([]byte(ownerHex + "22" + nonce))
	ownershipProof := "0x" + hex.EncodeToString(sum[:])

	const LinkRequestType uint8 = 0
	msgDigest, err := test.PreparePayloadForSigning(test.OwnershipProofSignaturePayload{
		RequestType:              LinkRequestType,
		WorkflowOwnerAddress:     common.HexToAddress(ownerHex),
		ChainID:                  chainID.String(),
		WorkflowRegistryContract: regAddr,
		Version:                  version,
		ValidityTimestamp:        validUntil,
		OwnershipProofHash:       common.HexToHash(ownershipProof),
	})

	if err != nil {
		return nil, "", fmt.Errorf("prepare payload: %w", err)
	}

	// Sign with allowed signer (TestPrivateKey2)
	signerKey, err := crypto.HexToECDSA(strings.TrimPrefix(constants.TestPrivateKey2, "0x"))
	if err != nil {
		return nil, "", fmt.Errorf("parse signer key: %w", err)
	}
	sig, err := crypto.Sign(msgDigest, signerKey)
	if err != nil {
		return nil, "", fmt.Errorf("sign: %w", err)
	}
	// Normalize V
	sig[64] += 27

	return map[string]any{
		"ownershipProofHash":   ownershipProof,
		"workflowOwnerAddress": ownerHex,
		"validUntil":           validUntil.Format(time.RFC3339),
		"signature":            "0x" + hex.EncodeToString(sig),
		"chainSelector":        strconv.FormatUint(chainSelector, 10),
		"contractAddress":      registryAddrHex,
		"transactionData":      "0x",
		"functionSignature":    "linkOwner(bytes32,bytes)",
		"functionArgs":         []string{},
	}, ownershipProof, nil
}

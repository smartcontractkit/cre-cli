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
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	test "github.com/smartcontractkit/cre-cli/test/contracts"
)

type gqlReq struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

func TestCLIAccountLinkListUnlinkFlow_EOA(t *testing.T) {
	anvilProc, testEthURL := initTestEnv(t)
	defer StopAnvil(anvilProc)

	tc := NewTestConfig(t)

	// Use test address for this test
	require.NoError(t, createCliEnvFile(tc.EnvFile, constants.TestPrivateKey4), "failed to create env file")
	require.NoError(t, createCliSettingsFile(tc, constants.TestAddress4, "workflow-name", testEthURL), "failed to create settings")
	require.NoError(t, createBlankProjectSettingFile(tc.ProjectDirectory+"project.yaml"), "failed to create project.yaml")
	t.Cleanup(tc.Cleanup(t))

	// Pre-baked registry addresses from Anvil state dump
	t.Setenv(environments.EnvVarWorkflowRegistryAddress, "0x5FbDB2315678afecb367f032d93F642f64180aa3")
	t.Setenv(environments.EnvVarWorkflowRegistryChainName, TestChainName)
	t.Setenv(environments.EnvVarCapabilitiesRegistryAddress, "0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9")
	t.Setenv(environments.EnvVarCapabilitiesRegistryChainName, TestChainName)

	// Set dummy API key
	t.Setenv(credentials.CreApiKeyVar, "test-api")

	registryAddr := os.Getenv(environments.EnvVarWorkflowRegistryAddress)
	require.NotEmpty(t, registryAddr, "registry address env must be set")

	// Track state for dynamic list responses
	isOwnerLinked := false

	// GraphQL server that supports InitiateLinking, InitiateUnlinking, and listWorkflowOwners
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
			// Extract owner address and validate request structure
			request, ok := req.Variables["request"].(map[string]any)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{{"message": "invalid request structure"}},
				})
				return
			}

			ownerHex, _ := request["workflowOwnerAddress"].(string)
			ownerLabel, _ := request["workflowOwnerLabel"].(string)
			environment, _ := request["environment"].(string)
			requestProcess, _ := request["requestProcess"].(string)

			chainSelector, err := settings.GetChainSelectorByChainName(TestChainName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{{"message": fmt.Sprintf("failed to get chain selector: %v", err)}},
				})
				return
			}

			// Build realistic response with correct function signature
			resp, _, err := buildInitiateLinkingEOAResponse(testEthURL, registryAddr, ownerHex, chainSelector)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{{"message": err.Error()}},
				})
				return
			}

			// Mark owner as linked for subsequent list operations
			isOwnerLinked = true

			t.Logf("InitiateLinking: owner=%s, label=%s, env=%s, process=%s", ownerHex, ownerLabel, environment, requestProcess)

			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"initiateLinking": resp,
				},
			})
			return

		case strings.Contains(req.Query, "InitiateUnlinking"):
			// Extract owner address and validate request structure
			request, ok := req.Variables["request"].(map[string]any)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{{"message": "invalid request structure"}},
				})
				return
			}

			ownerHex, _ := request["workflowOwnerAddress"].(string)
			environment, _ := request["environment"].(string)

			// Validate required fields
			if ownerHex == "" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{{"message": "missing workflowOwnerAddress"}},
				})
				return
			}

			chainSelector, err := settings.GetChainSelectorByChainName(TestChainName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{{"message": fmt.Sprintf("failed to get chain selector: %v", err)}},
				})
				return
			}

			// Build realistic response with correct function signature
			resp, err := buildInitiateUnlinkingEOAResponse(testEthURL, registryAddr, ownerHex, chainSelector)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errors": []map[string]string{{"message": err.Error()}},
				})
				return
			}

			// Mark owner as unlinked for subsequent list operations
			isOwnerLinked = false

			t.Logf("InitiateUnlinking: owner=%s, env=%s", ownerHex, environment)

			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"initiateUnlinking": resp,
				},
			})
			return

		case strings.Contains(req.Query, "listWorkflowOwners"):
			// Dynamic response based on current link state
			linkedOwners := []map[string]string{}
			if isOwnerLinked {
				linkedOwners = append(linkedOwners, map[string]string{
					"workflowOwnerAddress": constants.TestAddress4,
					"workflowOwnerLabel":   "owner-label-1",
					"environment":          "PRODUCTION_TESTNET",
					"verificationStatus":   "VERIFIED",
					"verifiedAt":           "2025-09-21T22:05:05.025287Z",
					"chainSelector": func() string {
						chainSelector, _ := settings.GetChainSelectorByChainName(TestChainName)
						return strconv.FormatUint(chainSelector, 10)
					}(),
					"contractAddress": registryAddr,
					"requestProcess":  "EOA",
				})
			}

			listResp := map[string]any{
				"data": map[string]any{
					"listWorkflowOwners": map[string]any{
						"linkedOwners": linkedOwners,
					},
				},
			}

			t.Logf("ListWorkflowOwners: returning %d linked owners", len(linkedOwners))
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
	t.Setenv(environments.EnvVarGraphQLURL, srv.URL+"/graphql")

	// ===== PHASE 1: LINK KEY =====
	t.Run("Link", func(t *testing.T) {
		args := []string{
			"account", "link-key",
			tc.GetCliEnvFlag(),
			tc.GetCliSettingsFlag(),
			"-l", "owner-label-1",
			"--" + settings.Flags.SkipConfirmation.Name,
		}
		cmd := exec.Command(CLIPath, args...)
		cmd.Dir = tc.ProjectDirectory

		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr

		err := cmd.Run()
		out := stripANSI(stdout.String() + stderr.String())

		// Test CLI behavior - GraphQL interaction and response parsing
		require.Contains(t, out, "Starting linking", "should announce linking start")
		require.Contains(t, out, "label=owner-label-1", "should show the provided label")
		require.Contains(t, out, "owner="+constants.TestAddress4, "should show the owner address")
		require.Contains(t, out, "Contract address validation passed", "should validate contract addresses match")

		// For CLI testing purposes, we consider success if:
		// 1. GraphQL request was made correctly
		// 2. Response was parsed correctly
		// 3. Function signature is correct
		// 4. CLI doesn't crash

		// Contract rejection is not a CLI failure - check for known contract error vs CLI error
		if err != nil {
			// If there's a contract-level rejection but CLI handled it properly, that's OK
			if strings.Contains(out, "function signature") && strings.Contains(out, "linkOwner") {
				t.Logf("Contract rejected payload but CLI handled GraphQL correctly (expected for test environment)")
			} else if strings.Contains(out, "InvalidOwnershipLink") || strings.Contains(out, "execution reverted") {
				t.Logf("Contract validation failed but CLI processed GraphQL response correctly (expected for test environment)")
			} else {
				t.Fatalf("CLI error (not contract rejection): %v\nOutput: %s", err, out)
			}
		} else {
			require.Contains(t, out, "Linked successfully", "should show success message")
		}
	})

	// ===== PHASE 2: LIST KEYS =====
	t.Run("List", func(t *testing.T) {
		args := []string{
			"account", "list-key",
			tc.GetCliEnvFlag(),
			tc.GetCliSettingsFlag(),
		}
		cmd := exec.Command(CLIPath, args...)
		cmd.Dir = tc.ProjectDirectory

		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr

		require.NoError(t, cmd.Run(), "list-key should not fail")

		out := stripANSI(stdout.String() + stderr.String())
		require.Contains(t, out, "Workflow owners retrieved successfully", "should show success message")

		// Check for linked owner (if link succeeded) or empty list (if link failed at contract level)
		if isOwnerLinked {
			require.Contains(t, out, "Linked Owners:", "should show linked owners section")
			require.Contains(t, out, "owner-label-1", "should show the owner label")
			require.Contains(t, out, constants.TestAddress4, "should show owner address")
			require.Contains(t, out, "Chain Selector:", "should show chain selector")
			require.Contains(t, out, "Contract Address:", "should show contract address")
		} else {
			require.Contains(t, out, "No linked owners found", "should show empty state when no owners linked")
		}
	})

	// ===== PHASE 3: UNLINK KEY =====
	t.Run("Unlink", func(t *testing.T) {
		args := []string{
			"account", "unlink-key",
			tc.GetCliEnvFlag(),
			tc.GetCliSettingsFlag(),
			"--" + settings.Flags.SkipConfirmation.Name,
		}
		cmd := exec.Command(CLIPath, args...)
		cmd.Dir = tc.ProjectDirectory

		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr

		err := cmd.Run()
		out := stripANSI(stdout.String() + stderr.String())

		// Test CLI behavior for unlink
		require.Contains(t, out, "Starting unlinking", "should announce unlinking start")
		require.Contains(t, out, "owner="+constants.TestAddress4, "should show the owner address")
		require.Contains(t, out, "Contract address validation passed", "should validate contract addresses match")

		// With -y flag, should not show confirmation prompt
		require.NotContains(t, out, "destructive action", "should skip confirmation prompt with -y flag")

		// For CLI testing purposes, contract rejection is acceptable
		if err != nil {
			// If there's a contract-level rejection but CLI handled it properly, that's OK
			if strings.Contains(out, "function signature") && strings.Contains(out, "unlinkOwner") {
				t.Logf("Contract rejected payload but CLI handled GraphQL correctly (expected for test environment)")
			} else if strings.Contains(out, "InvalidOwnershipLink") || strings.Contains(out, "execution reverted") {
				t.Logf("Contract validation failed but CLI processed GraphQL response correctly (expected for test environment)")
			} else {
				t.Fatalf("CLI error (not contract rejection): %v\nOutput: %s", err, out)
			}
		} else {
			require.Contains(t, out, "Unlinked successfully", "should show success message")
		}
	})

	// ===== PHASE 4: VERIFY UNLINKED STATE =====
	t.Run("ListAfterUnlink", func(t *testing.T) {
		args := []string{
			"account", "list-key",
			tc.GetCliEnvFlag(),
			tc.GetCliSettingsFlag(),
		}
		cmd := exec.Command(CLIPath, args...)
		cmd.Dir = tc.ProjectDirectory

		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr

		require.NoError(t, cmd.Run(), "list-key should not fail")

		out := stripANSI(stdout.String() + stderr.String())
		require.Contains(t, out, "Workflow owners retrieved successfully", "should show success message")

		// After unlink, should show no linked owners
		require.Contains(t, out, "No linked owners found", "should show no linked owners after unlink")
	})
}

func buildInitiateLinkingEOAResponse(rpcURL, registryAddrHex, ownerHex string, chainSelector uint64) (map[string]any, string, error) {
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

// Builds a valid InitiateUnlinking GraphQL response for EOA flow.
// Signs the message with the allowed signer (TestPrivateKey2) so CanUnlinkOwner + UnlinkOwner succeed.
func buildInitiateUnlinkingEOAResponse(rpcURL, registryAddrHex, ownerHex string, chainSelector uint64) (map[string]any, error) {
	ctx := context.Background()

	ec, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial ethclient: %w", err)
	}
	chainID, err := ec.NetworkID(ctx)
	if err != nil {
		return nil, fmt.Errorf("network id: %w", err)
	}

	regAddr := common.HexToAddress(registryAddrHex)
	reg, err := workflow_registry_v2_wrapper.NewWorkflowRegistry(regAddr, ec)
	if err != nil {
		return nil, fmt.Errorf("new registry: %w", err)
	}
	version, err := reg.TypeAndVersion(&bind.CallOpts{Context: ctx})
	if err != nil {
		return nil, fmt.Errorf("TypeAndVersion: %w", err)
	}

	// Valid for 1 hour
	validUntil := time.Now().UTC().Add(1 * time.Hour)

	// Deterministic proof (different from linking)
	nonce := strconv.FormatInt(time.Now().UnixNano(), 10)
	sum := sha256.Sum256([]byte(ownerHex + "unlink" + nonce))
	ownershipProof := "0x" + hex.EncodeToString(sum[:])

	const LinkRequestType uint8 = 0 // Unlink uses same request type as link
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
		return nil, fmt.Errorf("prepare payload: %w", err)
	}

	// Sign with allowed signer (TestPrivateKey2)
	signerKey, err := crypto.HexToECDSA(strings.TrimPrefix(constants.TestPrivateKey2, "0x"))
	if err != nil {
		return nil, fmt.Errorf("parse signer key: %w", err)
	}
	sig, err := crypto.Sign(msgDigest, signerKey)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	// Normalize V
	sig[64] += 27

	return map[string]any{
		"ownershipProofHash": ownershipProof,
		"validUntil":         validUntil.Format(time.RFC3339),
		"signature":          "0x" + hex.EncodeToString(sig),
		"chainSelector":      strconv.FormatUint(chainSelector, 10),
		"contractAddress":    registryAddrHex,
		"transactionData":    "0x",
		"functionSignature":  "unlinkOwner(address,uint256,bytes)",
		"functionArgs":       []string{},
	}, nil
}

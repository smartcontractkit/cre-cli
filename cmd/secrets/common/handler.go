package common

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/nacl/box"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	nautilus "github.com/smartcontractkit/chainlink-common/pkg/nodeauth/utils"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

// UpsertSecretsInputs holds the secrets passed to the CLI.
type UpsertSecretsInputs []SecretItem

// SecretItem represents a single secret with its ID, value, and optional namespace.
type SecretItem struct {
	ID        string `json:"id" validate:"required"`
	Value     string `json:"value" validate:"required"`
	Namespace string `json:"namespace"`
}

type Handler struct {
	Log             *zerolog.Logger
	ClientFactory   client.Factory
	SecretsFilePath string
	PrivateKey      *ecdsa.PrivateKey
	OwnerAddress    string
	EnvironmentSet  *environments.EnvironmentSet
	Gw              GatewayClient
}

// NewHandler creates a new handler instance.
func NewHandler(ctx *runtime.Context, secretsFilePath string) (*Handler, error) {
	var pk *ecdsa.PrivateKey
	var err error
	if ctx.Settings.User.EthPrivateKey != "" {
		pk, err = crypto.HexToECDSA(ctx.Settings.User.EthPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode the provided private key: %w", err)
		}
	} else {
		fmt.Println("No EthPrivateKey found in settings; assuming a multisig request.")
	}

	h := &Handler{
		Log:             ctx.Logger,
		ClientFactory:   ctx.ClientFactory,
		SecretsFilePath: secretsFilePath,
		PrivateKey:      pk,
		OwnerAddress:    ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		EnvironmentSet:  ctx.EnvironmentSet,
	}
	h.Gw = &HTTPClient{URL: h.EnvironmentSet.GatewayURL, Client: &http.Client{Timeout: 10 * time.Second}}
	return h, nil
}

// ResolveInputs unmarshals the JSON string into the UpsertSecretsInputs struct.
func (h *Handler) ResolveInputs() (UpsertSecretsInputs, error) {
	fileContent, err := os.ReadFile(h.SecretsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets file: %w", err)
	}

	var secrets UpsertSecretsInputs
	if err := json.Unmarshal(fileContent, &secrets); err != nil {
		return nil, fmt.Errorf("failed to parse JSON input from file: %w", err)
	}

	for i := range secrets {
		if secrets[i].Namespace == "" {
			secrets[i].Namespace = "main"
		}
	}
	return secrets, nil
}

// ValidateInputs validates the input structure.
func (h *Handler) ValidateInputs(inputs UpsertSecretsInputs) error {
	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to create validator: %w", err)
	}

	for i, item := range inputs {
		if err := validate.Struct(item); err != nil {
			return fmt.Errorf("validation failed for SecretItem at index %d: %w", i, err)
		}
	}

	return nil
}

// TODO: use TxType interface
func (h *Handler) PackAllowlistRequestTxData(reqDigestStr string, duration time.Duration) (string, error) {
	contractABI, err := abi.JSON(strings.NewReader(workflow_registry_wrapper_v2.WorkflowRegistryMetaData.ABI))
	if err != nil {
		return "", fmt.Errorf("failed to parse workflow registry v2 ABI: %w", err)
	}

	reqDigestBytes, err := client.HexToBytes32(reqDigestStr)
	if err != nil {
		h.Log.Error().Err(err).Msg("invalid request digest for AllowlistRequest")
		return "", fmt.Errorf("invalid request digest for AllowlistRequest: %w", err)
	}

	// #nosec G115 -- int64 to uint32 conversion; Unix() returns seconds since epoch, which fits in uint32 until 2106
	deadline := uint32(time.Now().Add(duration).Unix())

	data, err := contractABI.Pack("allowlistRequest", reqDigestBytes, deadline)
	if err != nil {
		return "", fmt.Errorf("failed to pack data for allowlistRequest: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func (h *Handler) LogMSIGNextSteps(txData string) error {
	fmt.Println("")
	fmt.Println("MSIG transaction prepared!")
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("")
	fmt.Println("   1. Submit the following transaction on the target chain:")
	fmt.Printf("      Chain:   %s\n", h.EnvironmentSet.WorkflowRegistryChainName)
	fmt.Printf("      Contract Address: %s\n", h.EnvironmentSet.WorkflowRegistryAddress)
	fmt.Println("")
	fmt.Println("   2. Use the following transaction data:")
	fmt.Println("")
	fmt.Printf("      %s\n", txData)
	fmt.Println("")
	fmt.Println("   3. Run the same command again without the --unsigned flag once transaction is finalized onchain")
	fmt.Println("")
	return nil
}

// EncryptSecrets takes the raw secrets and encrypts them, returning pointers.
func (h *Handler) EncryptSecrets(rawSecrets UpsertSecretsInputs) ([]*vault.EncryptedSecret, error) {
	capabilitiesRegistryClient, err := h.ClientFactory.NewCapabilitiesRegistryClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create capabilities registry client: %w", err)
	}

	// TODO instead of using cap registry, use gql MethodPublicKeyGet
	encryptionPublicKeyBytes, err := capabilitiesRegistryClient.GetVaultMasterPublicKey(constants.DefaultStagingDonFamily)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch master public key: %w", err)
	}

	if len(encryptionPublicKeyBytes) != 32 {
		return nil, fmt.Errorf("encryption public key has an invalid length: got %d bytes, want 32", len(encryptionPublicKeyBytes))
	}

	var encryptionPublicKey [32]byte
	copy(encryptionPublicKey[:], encryptionPublicKeyBytes)

	encryptedSecrets := make([]*vault.EncryptedSecret, 0, len(rawSecrets))

	for _, item := range rawSecrets {
		// encrypt
		plain := []byte(item.Value)
		cipher, err := box.SealAnonymous(nil, plain, &encryptionPublicKey, rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt secret: %w", err)
		}

		// build identifiers as pointers
		secID := &vault.SecretIdentifier{
			Key:       item.ID,
			Namespace: item.Namespace,
			Owner:     h.OwnerAddress,
		}

		encryptedSecrets = append(encryptedSecrets, &vault.EncryptedSecret{
			Id:             secID,
			EncryptedValue: hex.EncodeToString(cipher),
		})
	}

	return encryptedSecrets, nil
}

// Execute is a shared method for both 'create' and 'update' commands.
// It encapsulates the core logic of validation, encryption, and sending data.
func (h *Handler) Execute(inputs UpsertSecretsInputs, method string, duration time.Duration, ownerType string) error {
	// Encrypt the secrets
	encSecrets, err := h.EncryptSecrets(inputs)
	if err != nil {
		return fmt.Errorf("failed to encrypt secrets: %w", err)
	}

	var (
		requestBody []byte
		digest      string
	)

	switch method {
	case vaulttypes.MethodSecretsCreate:
		// Seed with empty RequestId
		seed := vault.CreateSecretsRequest{
			EncryptedSecrets: encSecrets,
		}

		// Generate the 16-char digest hash with the Request object with empty RequestId
		digest = nautilus.CalculateRequestDigest(&seed)

		requestID := uuid.New().String()
		// Build typed JSON-RPC request
		req := jsonrpc2.Request[vault.CreateSecretsRequest]{
			Version: jsonrpc2.JsonRpcVersion,
			ID:      requestID,
			Method:  method,
			Params: &vault.CreateSecretsRequest{
				RequestId:        requestID,
				EncryptedSecrets: encSecrets,
			},
		}

		requestBody, err = json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
		}

	case vaulttypes.MethodSecretsUpdate:
		// Seed with empty RequestId
		seed := vault.UpdateSecretsRequest{
			EncryptedSecrets: encSecrets,
		}

		// Generate the 16-char digest hash with the Request object with empty RequestId
		digest = nautilus.CalculateRequestDigest(&seed)

		requestID := uuid.New().String()
		// Build typed JSON-RPC request
		req := jsonrpc2.Request[vault.UpdateSecretsRequest]{
			Version: jsonrpc2.JsonRpcVersion,
			ID:      requestID,
			Method:  method,
			Params: &vault.UpdateSecretsRequest{
				RequestId:        requestID,
				EncryptedSecrets: encSecrets,
			},
		}

		requestBody, err = json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
		}

	default:
		return fmt.Errorf("unsupported method %q (expected \"vault.secrets.create\" or \"vault.secrets.update\")", method)
	}

	// if unsigned, prepare the tx data and return
	if ownerType == constants.WorkflowOwnerTypeMSIG {
		txData, err := h.PackAllowlistRequestTxData(digest, duration)
		if err != nil {
			return fmt.Errorf("failed to pack allowlist tx: %w", err)
		}
		if err := h.LogMSIGNextSteps(txData); err != nil {
			return fmt.Errorf("failed to log MSIG steps: %w", err)
		}
		return nil
	}

	// Register the digest on-chain
	wrV2Client, err := h.ClientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("create workflow registry client failed: %w", err)
	}
	ownerAddr := common.HexToAddress(h.OwnerAddress)
	allowlisted, err := wrV2Client.IsRequestAllowlisted(ownerAddr, digest)
	if err != nil {
		return fmt.Errorf("allowlist check failed: %w", err)
	}

	if !allowlisted {
		if err := wrV2Client.AllowlistRequest(digest, duration); err != nil {
			return fmt.Errorf("allowlist request failed: %w", err)
		}
		fmt.Printf("\nDigest allowlisted; proceeding to gateway POST: owner=%s, digest=%s\n", ownerAddr.Hex(), digest)
	} else {
		fmt.Printf("\nDigest already allowlisted; skipping on-chain allowlist: owner=%s, digest=%s\n", ownerAddr.Hex(), digest)
	}

	// POST to gateway
	respBody, status, err := h.Gw.Post(requestBody)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("gateway returned a non-200 status code: %d", status)
	}

	return h.ParseVaultGatewayResponse(method, respBody)
}

// ParseVaultGatewayResponse parses the JSON-RPC response, decodes the SignedOCRResponse payload
// into the appropriate proto type (CreateSecretsResponse, UpdateSecretsResponse, DeleteSecretsResponse),
// and logs one line per secret with id/owner/namespace/success/error.
func (h *Handler) ParseVaultGatewayResponse(method string, respBody []byte) error {
	// Unmarshal JSON-RPC envelope with SignedOCRResponse result
	var rpcResp jsonrpc2.Response[vaulttypes.SignedOCRResponse]
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return fmt.Errorf("failed to unmarshal JSON-RPC response: %w", err)
	}

	// JSON-RPC error?
	if rpcResp.Error != nil {
		b, _ := json.Marshal(rpcResp.Error)
		return fmt.Errorf("gateway returned JSON-RPC error: %s", string(b))
	}

	// Ensure we have a result payload
	if len(rpcResp.Result.Payload) == 0 {
		return fmt.Errorf("empty SignedOCRResponse payload")
	}

	// Decode OCR payload into the correct proto, print per-item results
	switch method {
	case vaulttypes.MethodSecretsCreate:
		var p vault.CreateSecretsResponse
		if err := protojson.Unmarshal(rpcResp.Result.Payload, &p); err != nil {
			return fmt.Errorf("failed to decode create payload: %w", err)
		}
		for _, r := range p.GetResponses() {
			id := r.GetId()
			// Safeguard for nil id
			key, owner, ns := "", "", ""
			if id != nil {
				key, owner, ns = id.GetKey(), id.GetOwner(), id.GetNamespace()
			}
			if r.GetSuccess() {
				fmt.Printf("Secret created: secret_id=%s, owner=%s, namespace=%s\n", key, owner, ns)
			} else {
				h.Log.Error().
					Str("secret_id", key).
					Str("owner", owner).
					Str("namespace", ns).
					Bool("success", false).
					Str("error", r.GetError()).
					Msg("secret create failed")
			}
		}
	case vaulttypes.MethodSecretsUpdate:
		var p vault.UpdateSecretsResponse
		if err := protojson.Unmarshal(rpcResp.Result.Payload, &p); err != nil {
			return fmt.Errorf("failed to decode update payload: %w", err)
		}
		for _, r := range p.GetResponses() {
			id := r.GetId()
			key, owner, ns := "", "", ""
			if id != nil {
				key, owner, ns = id.GetKey(), id.GetOwner(), id.GetNamespace()
			}
			if r.GetSuccess() {
				fmt.Printf("Secret updated: secret_id=%s, owner=%s, namespace=%s\n", key, owner, ns)
			} else {
				h.Log.Error().
					Str("secret_id", key).
					Str("owner", owner).
					Str("namespace", ns).
					Bool("success", false).
					Str("error", r.GetError()).
					Msg("secret update failed")
			}
		}
	case vaulttypes.MethodSecretsDelete:
		var p vault.DeleteSecretsResponse
		if err := protojson.Unmarshal(rpcResp.Result.Payload, &p); err != nil {
			return fmt.Errorf("failed to decode delete payload: %w", err)
		}
		for _, r := range p.GetResponses() {
			id := r.GetId()
			key, owner, ns := "", "", ""
			if id != nil {
				key, owner, ns = id.GetKey(), id.GetOwner(), id.GetNamespace()
			}
			if r.GetSuccess() {
				fmt.Printf("Secret deleted: secret_id=%s, owner=%s, namespace=%s\n", key, owner, ns)
			} else {
				h.Log.Error().
					Str("secret_id", key).
					Str("owner", owner).
					Str("namespace", ns).
					Bool("success", false).
					Str("error", r.GetError()).
					Msg("secret delete failed")
			}
		}
	case vaulttypes.MethodSecretsList:
		var p vault.ListSecretIdentifiersResponse
		if err := protojson.Unmarshal(rpcResp.Result.Payload, &p); err != nil {
			return fmt.Errorf("failed to decode list payload: %w", err)
		}

		if !p.GetSuccess() {
			h.Log.Error().
				Bool("success", false).
				Str("error", p.GetError()).
				Msg("list secrets failed")
			break
		}

		ids := p.GetIdentifiers()
		if len(ids) == 0 {
			fmt.Println("No secrets found")
			break
		}
		for _, id := range ids {
			key, owner, ns := "", "", ""
			if id != nil {
				key, owner, ns = id.GetKey(), id.GetOwner(), id.GetNamespace()
			}
			fmt.Printf("Secret identifier: secret_id=%s, owner=%s, namespace=%s\n", key, owner, ns)
		}
	default:
		// Unknown/unsupported method — don’t fail, just surface it explicitly
		h.Log.Warn().
			Str("method", method).
			Msg("received response for unsupported method; skipping payload decode")
	}

	return nil
}

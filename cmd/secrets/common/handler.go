package common

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	vaultcommon "github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"
	"github.com/smartcontractkit/tdh2/go/tdh2/tdh2easy"

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

type SecretsYamlConfig struct {
	SecretsNames map[string][]string `yaml:"secretsNames"`
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

	var cfg SecretsYamlConfig
	if err := yaml.Unmarshal(fileContent, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	if len(cfg.SecretsNames) == 0 {
		return nil, fmt.Errorf("YAML must contain a non-empty 'secretsNames' map")
	}

	out := make(UpsertSecretsInputs, 0, len(cfg.SecretsNames))

	for id, values := range cfg.SecretsNames {
		// Validate the ID’s UTF-8
		if !utf8.ValidString(id) {
			return nil, fmt.Errorf("secret id %q contains invalid UTF-8", id)
		}

		if len(values) == 0 {
			return nil, fmt.Errorf("secret %q has no values", id)
		}
		if len(values) != 1 {
			return nil, fmt.Errorf("secret %q must have exactly one env var name; got %d", id, len(values))
		}

		envName := strings.TrimSpace(values[0])
		if envName == "" {
			return nil, fmt.Errorf("secret %q has an empty env var name", id)
		}
		envVal, ok := os.LookupEnv(envName)
		if !ok {
			return nil, fmt.Errorf("environment variable %q for secret %q not found; please export it", envName, id)
		}

		// Validate the secret value’s UTF-8
		if !utf8.ValidString(envVal) {
			return nil, fmt.Errorf("value for secret %q (env %q) contains invalid UTF-8", id, envName)
		}

		out = append(out, SecretItem{
			ID:        id,
			Value:     envVal,
			Namespace: "main",
		})
	}
	return out, nil
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
func (h *Handler) PackAllowlistRequestTxData(reqDigest [32]byte, duration time.Duration) (string, error) {
	contractABI, err := abi.JSON(strings.NewReader(workflow_registry_wrapper_v2.WorkflowRegistryMetaData.ABI))
	if err != nil {
		return "", fmt.Errorf("failed to parse workflow registry v2 ABI: %w", err)
	}

	// #nosec G115 -- int64 to uint32 conversion; Unix() returns seconds since epoch, which fits in uint32 until 2106
	deadline := uint32(time.Now().Add(duration).Unix())

	data, err := contractABI.Pack("allowlistRequest", reqDigest, deadline)
	if err != nil {
		return "", fmt.Errorf("failed to pack data for allowlistRequest: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func (h *Handler) LogMSIGNextSteps(txData string, requestID string) error {
	fmt.Println("")
	fmt.Println("MSIG transaction prepared!")
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("")
	fmt.Println("   1. Submit the following transaction on the target chain:")
	fmt.Printf("      Chain:            %s\n", h.EnvironmentSet.WorkflowRegistryChainName)
	fmt.Printf("      Contract Address: %s\n", h.EnvironmentSet.WorkflowRegistryAddress)
	fmt.Println("")
	fmt.Println("   2. Use the following transaction data:")
	fmt.Println("")
	fmt.Printf("      %s\n", txData)
	fmt.Println("")
	fmt.Println("   3. Save these values; you will need them on the second run:")
	fmt.Printf("      Request ID: %s\n", requestID)
	fmt.Println("")
	fmt.Println("   4. After the transaction is finalized on-chain, run the SAME command again,")
	fmt.Println("      adding the --request-id flag with the value above, e.g. for create:")
	fmt.Println("")
	fmt.Println("      cre secrets create <secrets-file> --request-id=", requestID)
	fmt.Println("")
	return nil
}

// EncryptSecrets takes the raw secrets and encrypts them, returning pointers.
func (h *Handler) EncryptSecrets(rawSecrets UpsertSecretsInputs) ([]*vault.EncryptedSecret, error) {
	requestID := uuid.New().String()
	getPublicKeyRequest := jsonrpc2.Request[vaultcommon.GetPublicKeyRequest]{
		Version: jsonrpc2.JsonRpcVersion,
		ID:      requestID,
		Method:  vaulttypes.MethodPublicKeyGet,      // "vault_publicKey_get"
		Params:  &vaultcommon.GetPublicKeyRequest{}, // empty payload per current API
	}

	reqBody, err := json.Marshal(getPublicKeyRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key request: %w", err)
	}

	respBody, status, err := h.Gw.Post(reqBody)
	if err != nil {
		return nil, fmt.Errorf("gateway POST failed: %w", err)
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("gateway returned non-200: %d body=%s", status, string(respBody))
	}

	var rpcResp jsonrpc2.Response[vaultcommon.GetPublicKeyResponse]
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal public key response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("vault public key fetch error: %s", rpcResp.Error.Error())
	}
	if rpcResp.Version != jsonrpc2.JsonRpcVersion {
		return nil, fmt.Errorf("jsonrpc version mismatch: got %q", rpcResp.Version)
	}
	if rpcResp.ID != requestID {
		return nil, fmt.Errorf("jsonrpc id mismatch: got %q want %q", rpcResp.ID, requestID)
	}
	if rpcResp.Method != vaulttypes.MethodPublicKeyGet {
		return nil, fmt.Errorf("jsonrpc method mismatch: got %q", rpcResp.Method)
	}
	if rpcResp.Result == nil || rpcResp.Result.PublicKey == "" {
		return nil, fmt.Errorf("empty result in public key response")
	}

	pubKeyHex := rpcResp.Result.PublicKey // already hex per gateway

	// Encrypt each secret with tdh2easy
	encryptedSecrets := make([]*vault.EncryptedSecret, 0, len(rawSecrets))
	for _, item := range rawSecrets {
		cipherHex, err := EncryptSecret(item.Value, pubKeyHex)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt secret (key=%s ns=%s): %w", item.ID, item.Namespace, err)
		}

		secID := &vault.SecretIdentifier{
			Key:       item.ID,
			Namespace: item.Namespace,
			Owner:     h.OwnerAddress,
		}

		encryptedSecrets = append(encryptedSecrets, &vault.EncryptedSecret{
			Id:             secID,
			EncryptedValue: cipherHex,
		})
	}

	return encryptedSecrets, nil
}

func EncryptSecret(secret, masterPublicKeyHex string) (string, error) {
	masterPublicKey := tdh2easy.PublicKey{}
	masterPublicKeyBytes, err := hex.DecodeString(masterPublicKeyHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode master public key: %w", err)
	}
	if err = masterPublicKey.Unmarshal(masterPublicKeyBytes); err != nil {
		return "", fmt.Errorf("failed to unmarshal master public key: %w", err)
	}
	cipher, err := tdh2easy.Encrypt(&masterPublicKey, []byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to encrypt secret: %w", err)
	}
	cipherBytes, err := cipher.Marshal()
	if err != nil {
		return "", fmt.Errorf("failed to marshal encrypted secrets to bytes: %w", err)
	}
	return hex.EncodeToString(cipherBytes), nil
}

func CalculateDigest[I any](r jsonrpc2.Request[I]) ([32]byte, error) {
	b, err := json.Marshal(r.Params)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to marshal json request params: %w", err)
	}

	req := jsonrpc2.Request[json.RawMessage]{
		Version: r.Version,
		ID:      r.ID,
		Method:  r.Method,
		Params:  (*json.RawMessage)(&b),
	}

	digestStr, err := req.Digest()
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to calculate digest: %w", err)
	}

	digestBytes32, err := HexToBytes32(digestStr)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to convert digest hex to [32]byte: %w", err)
	}

	return digestBytes32, nil
}

// HexToBytes32 converts a hex string (with or without 0x prefix) to a [32]byte.
// Returns an error if the input isn't precisely 32 bytes after decoding.
func HexToBytes32(h string) ([32]byte, error) {
	var out [32]byte
	h = strings.TrimPrefix(h, "0x")
	b, err := hex.DecodeString(h)
	if err != nil {
		return out, fmt.Errorf("invalid hex for digest: %w", err)
	}
	if len(b) != 32 {
		return out, fmt.Errorf("digest must be 32 bytes, got %d", len(b))
	}
	copy(out[:], b)
	return out, nil
}

// Execute is a shared method for both 'create' and 'update' commands.
// It encapsulates the core logic of validation, encryption, and sending data.
func (h *Handler) Execute(
	inputs UpsertSecretsInputs,
	method string,
	duration time.Duration,
	ownerType string,
	requestIDFlag string,
) error {
	// Encrypt the secrets
	encSecrets, err := h.EncryptSecrets(inputs)
	if err != nil {
		return fmt.Errorf("failed to encrypt secrets: %w", err)
	}

	// Resolve request ID (reuse if provided)
	var requestID string
	rid := strings.TrimSpace(requestIDFlag)
	if rid == "" {
		requestID = uuid.New().String()
	} else {
		id, err := uuid.Parse(rid)
		if err != nil {
			return fmt.Errorf("--request-id must be a valid UUID: %w", err)
		}
		requestID = id.String() // canonical form
	}

	var (
		requestBody []byte
		digest      [32]byte
	)

	switch method {
	case vaulttypes.MethodSecretsCreate:
		req := jsonrpc2.Request[vault.CreateSecretsRequest]{
			Version: jsonrpc2.JsonRpcVersion,
			ID:      requestID,
			Method:  method,
			Params: &vault.CreateSecretsRequest{
				RequestId:        requestID,
				EncryptedSecrets: encSecrets,
			},
		}
		d, err := CalculateDigest(req)
		if err != nil {
			return fmt.Errorf("failed to calculate create digest: %w", err)
		}
		digest = d
		requestBody, err = json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
		}

	case vaulttypes.MethodSecretsUpdate:
		req := jsonrpc2.Request[vault.UpdateSecretsRequest]{
			Version: jsonrpc2.JsonRpcVersion,
			ID:      requestID,
			Method:  method,
			Params: &vault.UpdateSecretsRequest{
				RequestId:        requestID,
				EncryptedSecrets: encSecrets,
			},
		}
		d, err := CalculateDigest(req)
		if err != nil {
			return fmt.Errorf("failed to calculate update digest: %w", err)
		}
		digest = d
		requestBody, err = json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
		}

	default:
		return fmt.Errorf("unsupported method %q (expected %q or %q)", method, vaulttypes.MethodSecretsCreate, vaulttypes.MethodSecretsUpdate)
	}

	// MSIG first run: owner is MSIG and no request-id. Only print steps & exit.
	if ownerType == constants.WorkflowOwnerTypeMSIG && rid == "" {
		txData, err := h.PackAllowlistRequestTxData(digest, duration)
		if err != nil {
			return fmt.Errorf("failed to pack allowlist tx: %w", err)
		}
		if err := h.LogMSIGNextSteps(txData, requestID); err != nil {
			return fmt.Errorf("failed to log MSIG steps: %w", err)
		}
		return nil
	}

	// From here on, we're in the "call the DON" path.
	// If --request-id is provided for ANY owner type: do NOT allowlist; only proceed if allowlisted.
	// Else (EOA path): auto-allowlist if needed.
	wrV2Client, err := h.ClientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("create workflow registry client failed: %w", err)
	}
	ownerAddr := common.HexToAddress(h.OwnerAddress)

	allowlisted, err := wrV2Client.IsRequestAllowlisted(ownerAddr, digest)
	if err != nil {
		return fmt.Errorf("allowlist check failed: %w", err)
	}

	if rid == "" {
		// first time run: no --request-id
		if !allowlisted {
			if err := wrV2Client.AllowlistRequest(digest, duration); err != nil {
				return fmt.Errorf("allowlist request failed: %w", err)
			}
			fmt.Printf("Digest allowlisted; proceeding to gateway POST: owner=%s, requestID=%s, digest=0x%x\n", ownerAddr.Hex(), requestID, digest)
		} else {
			fmt.Printf("Digest already allowlisted; skipping on-chain allowlist: owner=%s, requestID=%s, digest=0x%x\n", ownerAddr.Hex(), requestID, digest)
		}
	} else {
		// second time run: users provided --request-id
		if !allowlisted {
			return fmt.Errorf("on-chain request for request-id %q is not finalized (digest not allowlisted). Do not call the vault DON yet. Finalize the on-chain allowlist tx, then rerun this command with the same --request-id", requestIDFlag)
		}
		fmt.Printf("Digest allowlisted; proceeding to gateway POST: owner=%s, requestID=%s, digest=0x%x\n", ownerAddr.Hex(), requestID, digest)

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
				fmt.Printf("Secret create failed: secret_id=%s owner=%s namespace=%s success=%t error=%s\n",
					key, owner, ns, false, r.GetError(),
				)
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
				fmt.Printf("Secret update failed: secret_id=%s owner=%s namespace=%s success=%t error=%s\n",
					key, owner, ns, false, r.GetError(),
				)
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
				fmt.Printf("Secret delete failed: secret_id=%s owner=%s namespace=%s success=%t error=%s\n",
					key, owner, ns, false, r.GetError(),
				)
			}
		}
	case vaulttypes.MethodSecretsList:
		var p vault.ListSecretIdentifiersResponse
		if err := protojson.Unmarshal(rpcResp.Result.Payload, &p); err != nil {
			return fmt.Errorf("failed to decode list payload: %w", err)
		}

		if !p.GetSuccess() {
			fmt.Printf("secret list failed: success=%t error=%s\n",
				false, p.GetError(),
			)
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

package common

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"
	"github.com/smartcontractkit/tdh2/go/tdh2/tdh2easy"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/types"
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
	Wrc             *client.WorkflowRegistryV2Client
	Credentials     *credentials.Credentials
	Settings        *settings.Settings
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
		ctx.Logger.Debug().Msg("No EthPrivateKey found in settings; assuming a multisig request.")

	}

	h := &Handler{
		Log:             ctx.Logger,
		ClientFactory:   ctx.ClientFactory,
		SecretsFilePath: secretsFilePath,
		PrivateKey:      pk,
		OwnerAddress:    ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		EnvironmentSet:  ctx.EnvironmentSet,
		Credentials:     ctx.Credentials,
		Settings:        ctx.Settings,
	}
	h.Gw = &HTTPClient{URL: h.EnvironmentSet.GatewayURL, Client: &http.Client{Timeout: 90 * time.Second}}

	wrc, err := h.ClientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow registry client: %w", err)
	}
	h.Wrc = wrc

	return h, nil
}

// ResolveInputs loads secrets from a YAML file.
// Errors if the path is not .yaml/.yml — MSIG step 2 is handled by `cre secrets execute`.
func (h *Handler) ResolveInputs() (UpsertSecretsInputs, error) {
	ext := strings.ToLower(filepath.Ext(h.SecretsFilePath))
	if ext != ".yaml" && ext != ".yml" {
		return nil, fmt.Errorf("expected a YAML file; for MSIG step 2 use `cre secrets execute <bundle.json>`")
	}

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
		if !utf8.ValidString(envVal) {
			return nil, fmt.Errorf("value for secret %q (env %q) contains invalid UTF-8", id, envName)
		}

		out = append(out, SecretItem{
			ID:        id,
			Value:     envVal,
			Namespace: "main",
		})

		// Enforce max payload size of 10 items.
		if len(out) > constants.MaxSecretItemsPerPayload {
			return nil, fmt.Errorf("cannot have more than 10 items in a single payload; check your secrets YAML")
		}
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

	// #nosec G115
	deadline := uint32(time.Now().Add(duration).Unix())

	data, err := contractABI.Pack("allowlistRequest", reqDigest, deadline)
	if err != nil {
		return "", fmt.Errorf("failed to pack data for allowlistRequest: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func (h *Handler) LogMSIGNextSteps(txData string, digest [32]byte, bundlePath string) error {
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
	fmt.Println("   3. Save this bundle file; you will need it on the second run:")
	fmt.Printf("      Bundle Path: %s\n", bundlePath)
	fmt.Printf("      Digest:      0x%s\n", hex.EncodeToString(digest[:]))
	fmt.Println("")
	fmt.Println("   4. After the transaction is finalized on-chain, run:")
	fmt.Println("")
	fmt.Println("      cre secrets execute", bundlePath, "--unsigned")
	fmt.Println("")
	return nil
}

// EncryptSecrets takes the raw secrets and encrypts them, returning pointers.
func (h *Handler) EncryptSecrets(rawSecrets UpsertSecretsInputs) ([]*vault.EncryptedSecret, error) {
	requestID := uuid.New().String()
	getPublicKeyRequest := jsonrpc2.Request[vault.GetPublicKeyRequest]{
		Version: jsonrpc2.JsonRpcVersion,
		ID:      requestID,
		Method:  vaulttypes.MethodPublicKeyGet,
		Params:  &vault.GetPublicKeyRequest{},
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

	var rpcResp jsonrpc2.Response[vault.GetPublicKeyResponse]
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

	pubKeyHex := rpcResp.Result.PublicKey

	encryptedSecrets := make([]*vault.EncryptedSecret, 0, len(rawSecrets))
	for _, item := range rawSecrets {
		cipherHex, err := EncryptSecret(item.Value, pubKeyHex, h.OwnerAddress)
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

func EncryptSecret(secret, masterPublicKeyHex string, ownerAddress string) (string, error) {
	masterPublicKey := tdh2easy.PublicKey{}
	masterPublicKeyBytes, err := hex.DecodeString(masterPublicKeyHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode master public key: %w", err)
	}
	if err = masterPublicKey.Unmarshal(masterPublicKeyBytes); err != nil {
		return "", fmt.Errorf("failed to unmarshal master public key: %w", err)
	}

	addr := common.HexToAddress(ownerAddress) // canonical 20-byte address
	var label [32]byte
	copy(label[12:], addr.Bytes()) // left-pad with 12 zero bytes
	cipher, err := tdh2easy.EncryptWithLabel(&masterPublicKey, []byte(secret), label)
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

// Execute is shared for 'create' and 'update' (YAML-only).
// - MSIG => step 1: build request, save bundle, print instructions
// - EOA  => build request, allowlist if needed, POST
func (h *Handler) Execute(
	inputs UpsertSecretsInputs,
	method string,
	duration time.Duration,
	ownerType string,
) error {
	fmt.Println("Verifying ownership...")
	if err := h.EnsureOwnerLinkedOrFail(); err != nil {
		return err
	}

	// Build from YAML inputs
	encSecrets, err := h.EncryptSecrets(inputs)
	if err != nil {
		return fmt.Errorf("failed to encrypt secrets: %w", err)
	}
	requestID := uuid.New().String()

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
		if digest, err = CalculateDigest(req); err != nil {
			return fmt.Errorf("failed to calculate create digest: %w", err)
		}
		if requestBody, err = json.Marshal(req); err != nil {
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
		if digest, err = CalculateDigest(req); err != nil {
			return fmt.Errorf("failed to calculate update digest: %w", err)
		}
		if requestBody, err = json.Marshal(req); err != nil {
			return fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
		}

	default:
		return fmt.Errorf("unsupported method %q (expected %q or %q)", method, vaulttypes.MethodSecretsCreate, vaulttypes.MethodSecretsUpdate)
	}

	ownerAddr := common.HexToAddress(h.OwnerAddress)

	allowlisted, err := h.Wrc.IsRequestAllowlisted(ownerAddr, digest)
	if err != nil {
		return fmt.Errorf("allowlist check failed: %w", err)
	}
	var txOut *client.TxOutput
	if !allowlisted {
		if txOut, err = h.Wrc.AllowlistRequest(digest, duration); err != nil {
			return fmt.Errorf("allowlist request failed: %w", err)
		}
	}

	gatewayPost := func() error {
		respBody, status, err := h.Gw.Post(requestBody)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("gateway returned a non-200 status code: %d", status)
		}
		return h.ParseVaultGatewayResponse(method, respBody)
	}

	if txOut == nil && allowlisted {
		fmt.Printf("Digest already allowlisted; proceeding to gateway POST: owner=%s, digest=0x%x\n", ownerAddr.Hex(), digest)
		return gatewayPost()
	}

	baseDir := filepath.Dir(h.SecretsFilePath)
	filename := DeriveBundleFilename(digest) // <digest>.json
	bundlePath := filepath.Join(baseDir, filename)

	ub := &UnsignedBundle{
		RequestID:   requestID,
		Method:      method,
		DigestHex:   "0x" + hex.EncodeToString(digest[:]),
		RequestBody: requestBody,
		CreatedAt:   time.Now().UTC(),
	}

	switch txOut.Type {
	case client.Regular:
		fmt.Println("Transaction confirmed")
		fmt.Printf("Digest allowlisted; proceeding to gateway POST: owner=%s, digest=0x%x\n", ownerAddr.Hex(), digest)
		fmt.Printf("View on explorer: \033]8;;%s/tx/%s\033\\%s/tx/%s\033]8;;\033\\\n", h.EnvironmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash, h.EnvironmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash)
		return gatewayPost()
	case client.Raw:
		if err := SaveBundle(bundlePath, ub); err != nil {
			return fmt.Errorf("failed to save unsigned bundle at %s: %w", bundlePath, err)
		}

		txData, err := h.PackAllowlistRequestTxData(digest, duration)
		if err != nil {
			return fmt.Errorf("failed to pack allowlist tx: %w", err)
		}
		return h.LogMSIGNextSteps(txData, digest, bundlePath)
	case client.Changeset:
		chainSelector, err := settings.GetChainSelectorByChainName(h.EnvironmentSet.WorkflowRegistryChainName)
		if err != nil {
			return fmt.Errorf("failed to get chain selector for chain %q: %w", h.EnvironmentSet.WorkflowRegistryChainName, err)
		}
		mcmsConfig, err := settings.GetMCMSConfig(h.Settings, chainSelector)
		if err != nil {
			fmt.Println("\nMCMS config not found or is incorrect, skipping MCMS config in changeset")
		}
		cldSettings := h.Settings.CLDSettings
		changesets := []types.Changeset{
			{
				AllowlistRequest: &types.AllowlistRequest{
					Payload: types.UserAllowlistRequestInput{
						ExpiryTimestamp:           uint32(time.Now().Add(duration).Unix()), // #nosec G115 -- int64 to uint32 conversion; Unix() returns seconds since epoch, which fits in uint32 until 2106
						RequestDigest:             common.Bytes2Hex(digest[:]),
						ChainSelector:             chainSelector,
						MCMSConfig:                mcmsConfig,
						WorkflowRegistryQualifier: cldSettings.WorkflowRegistryQualifier,
					},
				},
			},
		}
		csFile := types.NewChangesetFile(cldSettings.Environment, cldSettings.Domain, cldSettings.MergeProposals, changesets)

		var fileName string
		if cldSettings.ChangesetFile != "" {
			fileName = cldSettings.ChangesetFile
		} else {
			fileName = fmt.Sprintf("AllowlistRequest_%s_%s_%s.yaml", requestID, h.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress, time.Now().Format("20060102_150405"))
		}

		if err := SaveBundle(bundlePath, ub); err != nil {
			return fmt.Errorf("failed to save unsigned bundle at %s: %w", bundlePath, err)
		}

		return cmdCommon.WriteChangesetFile(fileName, csFile, h.Settings)

	default:
		h.Log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
	}
	return nil
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

// EnsureOwnerLinkedOrFail TODO this reuses the same logic as in autoLink.go which is tied to deploy; consider refactoring to avoid duplication
func (h *Handler) EnsureOwnerLinkedOrFail() error {
	ownerAddr := common.HexToAddress(h.OwnerAddress)

	linked, err := h.Wrc.IsOwnerLinked(ownerAddr)
	if err != nil {
		return fmt.Errorf("failed to check owner link status: %w", err)
	}

	fmt.Printf("Workflow owner link status: owner=%s, linked=%v\n", ownerAddr.Hex(), linked)

	if linked {
		// Owner is linked on contract, now verify it's linked to the current user's account
		linkedToCurrentUser, err := h.checkLinkStatusViaGraphQL(ownerAddr)
		if err != nil {
			return fmt.Errorf("failed to validate key ownership: %w", err)
		}

		if !linkedToCurrentUser {
			return fmt.Errorf("key %s is linked to another account. Please use a different owner address", ownerAddr.Hex())
		}

		fmt.Println("Key ownership verified")
		return nil
	}

	return fmt.Errorf("owner %s not linked; run cre account link-key", ownerAddr.Hex())
}

// checkLinkStatusViaGraphQL checks if the owner is linked and verified by querying the service
func (h *Handler) checkLinkStatusViaGraphQL(ownerAddr common.Address) (bool, error) {
	const query = `
	query {
		listWorkflowOwners(filters: { linkStatus: LINKED_ONLY }) {
			linkedOwners {
				workflowOwnerAddress
				verificationStatus
			}
		}
	}`

	req := graphql.NewRequest(query)
	var resp struct {
		ListWorkflowOwners struct {
			LinkedOwners []struct {
				WorkflowOwnerAddress string `json:"workflowOwnerAddress"`
				VerificationStatus   string `json:"verificationStatus"`
			} `json:"linkedOwners"`
		} `json:"listWorkflowOwners"`
	}

	gql := graphqlclient.New(h.Credentials, h.EnvironmentSet, h.Log)
	if err := gql.Execute(context.Background(), req, &resp); err != nil {
		return false, fmt.Errorf("GraphQL query failed: %w", err)
	}

	ownerHex := strings.ToLower(ownerAddr.Hex())
	for _, linkedOwner := range resp.ListWorkflowOwners.LinkedOwners {
		if strings.ToLower(linkedOwner.WorkflowOwnerAddress) == ownerHex {
			// Check if verification status is successful
			//nolint:misspell // Intentional misspelling to match external API
			if linkedOwner.VerificationStatus == "VERIFICATION_STATUS_SUCCESSFULL" {
				h.Log.Debug().
					Str("ownerAddress", linkedOwner.WorkflowOwnerAddress).
					Str("verificationStatus", linkedOwner.VerificationStatus).
					Msg("Owner found and verified")
				return true, nil
			}
			h.Log.Debug().
				Str("ownerAddress", linkedOwner.WorkflowOwnerAddress).
				Str("verificationStatus", linkedOwner.VerificationStatus).
				Str("expectedStatus", "VERIFICATION_STATUS_SUCCESSFULL"). //nolint:misspell // Intentional misspelling to match external API
				Msg("Owner found but verification status not successful")
			return false, nil
		}
	}

	h.Log.Debug().
		Str("ownerAddress", ownerAddr.Hex()).
		Msg("Owner not found in linked owners list")

	return false, nil
}

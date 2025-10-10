package delete

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	nautilus "github.com/smartcontractkit/chainlink-common/pkg/nodeauth/utils"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

// DeleteSecretItem represents a single secret to be deleted with its ID and optional namespace.
type DeleteSecretItem struct {
	ID        string `json:"id" validate:"required"`
	Namespace string `json:"namespace"`
}

// DeleteSecretsInputs holds the secrets to be deleted.
type DeleteSecretsInputs []DeleteSecretItem

// secretsNames:
//   - SECRET_NAME1
//   - SECRET_NAME2
type SecretsDeleteYamlConfig struct {
	SecretsNames []string `yaml:"secretsNames"`
}

// New creates and returns the 'secrets delete' cobra command.
func New(ctx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [SECRETS_FILE_PATH]",
		Short:   "Deletes secrets from a JSON file provided as a positional argument.",
		Example: "cre secrets delete my-secrets.json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			secretsFilePath := args[0]

			h, err := common.NewHandler(ctx, secretsFilePath)
			if err != nil {
				return err
			}

			duration, err := cmd.Flags().GetDuration("timeout")
			if err != nil {
				return err
			}

			maxDuration := constants.MaxVaultAllowlistDuration
			maxHours := int(maxDuration / time.Hour)
			maxDays := int(maxDuration / (24 * time.Hour))
			if duration <= 0 || duration > maxDuration {
				ctx.Logger.Error().
					Dur("timeout", duration).
					Dur("maxDuration", maxDuration).
					Msg(fmt.Sprintf("invalid timeout: must be > 0 and < %dh (%dd)", maxHours, maxDays))

				return fmt.Errorf("invalid --timeout: must be greater than 0 and less than %dh (%dd)", maxHours, maxDays)
			}

			inputs, err := ResolveDeleteInputs(secretsFilePath)
			if err != nil {
				return err
			}

			if err := ValidateDeleteInputs(inputs); err != nil {
				return err
			}

			return Execute(h, inputs, duration, ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType)
		},
	}

	settings.AddRawTxFlag(cmd)

	return cmd
}

// Execute handles the main logic for the 'delete' command.
func Execute(h *common.Handler, inputs DeleteSecretsInputs, duration time.Duration, ownerType string) error {
	// Validate and canonicalize owner address
	owner := strings.TrimSpace(h.OwnerAddress)
	if !ethcommon.IsHexAddress(owner) {
		return fmt.Errorf("invalid owner address: %q", h.OwnerAddress)
	}
	owner = ethcommon.HexToAddress(owner).Hex() // checksummed string

	// Prepare the list of SecretIdentifiers to be deleted.
	ptrIDs := make([]*vault.SecretIdentifier, len(inputs))
	for i, item := range inputs {
		ptrIDs[i] = &vault.SecretIdentifier{
			Key:       item.ID,
			Namespace: item.Namespace,
			Owner:     owner,
		}
	}

	seed := vault.DeleteSecretsRequest{
		Ids: ptrIDs, // order is significant
	}
	// Generate the 16-char digest hash, this is the Request object with empty RequestId
	digest := nautilus.CalculateRequestDigest(&seed)

	requestID := uuid.New().String()
	// Use the ID and prepare the JSON RPC request
	deleteSecretsRequest := jsonrpc2.Request[vault.DeleteSecretsRequest]{
		Version: jsonrpc2.JsonRpcVersion,
		ID:      requestID,
		Method:  vaulttypes.MethodSecretsDelete,
		Params: &vault.DeleteSecretsRequest{
			RequestId: requestID,
			Ids:       ptrIDs,
		},
	}

	requestBody, err := json.Marshal(deleteSecretsRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
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
	// TODO double check: for the 2nd step of MSIG, we shouldnt require private key and shouldnt require unsigned flag

	// Register the digest on-chain
	wrV2Client, err := h.ClientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("create workflow registry client failed: %w", err)
	}
	ownerAddr := ethcommon.HexToAddress(h.OwnerAddress)
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

	return h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsDelete, respBody)

}

// ResolveDeleteInputs unmarshals the JSON string into the DeleteSecretsInputs struct.
func ResolveDeleteInputs(secretsFilePath string) (DeleteSecretsInputs, error) {
	fileContent, err := os.ReadFile(secretsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets file: %w", err)
	}

	var cfg SecretsDeleteYamlConfig
	if err := yaml.Unmarshal(fileContent, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	if len(cfg.SecretsNames) == 0 {
		return nil, fmt.Errorf("YAML must contain a non-empty 'secretsNames' list")
	}

	out := make(DeleteSecretsInputs, 0, len(cfg.SecretsNames))
	for _, id := range cfg.SecretsNames {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("'secretsNames' list contains an empty id")
		}
		// Validate the ID’s UTF-8
		if !utf8.ValidString(id) {
			return nil, fmt.Errorf("secret id %q contains invalid UTF-8", id)
		}

		out = append(out, DeleteSecretItem{
			ID:        id,
			Namespace: "main",
		})
	}
	return out, nil
}

// ValidateDeleteInputs validates the delete input structure.
func ValidateDeleteInputs(inputs DeleteSecretsInputs) error {
	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to create validator: %w", err)
	}

	if len(inputs) == 0 {
		return fmt.Errorf("no secrets provided: file contains empty array")
	}

	for i, item := range inputs {
		if err := validate.Struct(item); err != nil {
			return fmt.Errorf("validation failed for SecretItem at index %d: %w", i, err)
		}
	}
	return nil
}

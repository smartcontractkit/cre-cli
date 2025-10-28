package list

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// cre secrets list --timeout 1h
func New(ctx *runtime.Context) *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists secret identifiers for the current owner address in the given namespace.",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := common.NewHandler(ctx, "")
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

			return Execute(
				h,
				namespace,
				duration,
				ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType,
			)
		},
	}

	cmd.Flags().StringVar(&namespace, "namespace", "main", "Namespace to list (default: main)")
	settings.AddRawTxFlag(cmd)

	return cmd
}

// Execute performs: build request → (MSIG step 1 bundle OR EOA allowlist+post) → parse.
func Execute(h *common.Handler, namespace string, duration time.Duration, ownerType string) error {
	fmt.Println("Verifying ownership...")
	if err := h.EnsureOwnerLinkedOrFail(); err != nil {
		return err
	}

	if namespace == "" {
		namespace = "main"
	}

	// Validate and canonicalize owner address (checksummed)
	owner := strings.TrimSpace(h.OwnerAddress)
	if !ethcommon.IsHexAddress(owner) {
		return fmt.Errorf("invalid owner address: %q", h.OwnerAddress)
	}
	owner = ethcommon.HexToAddress(owner).Hex()

	// Fresh request ID
	requestID := uuid.New().String()

	req := jsonrpc2.Request[vault.ListSecretIdentifiersRequest]{
		Version: jsonrpc2.JsonRpcVersion,
		ID:      requestID,
		Method:  vaulttypes.MethodSecretsList,
		Params: &vault.ListSecretIdentifiersRequest{
			RequestId: requestID,
			Owner:     owner,
			Namespace: namespace,
		},
	}

	digest, err := common.CalculateDigest(req)
	if err != nil {
		return fmt.Errorf("failed to calculate request digest: %w", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
	}

	// ---------------- MSIG step 1: bundle and exit ----------------
	if ownerType == constants.WorkflowOwnerTypeMSIG {
		// Save bundle in the current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		filename := common.DeriveBundleFilename(digest) // <digest>.json
		bundlePath := filepath.Join(cwd, filename)

		ub := &common.UnsignedBundle{
			RequestID:   requestID,
			Method:      vaulttypes.MethodSecretsList,
			DigestHex:   "0x" + hex.EncodeToString(digest[:]),
			RequestBody: body,
			CreatedAt:   time.Now().UTC(),
		}
		if err := common.SaveBundle(bundlePath, ub); err != nil {
			return fmt.Errorf("failed to save unsigned bundle at %s: %w", bundlePath, err)
		}

		txData, err := h.PackAllowlistRequestTxData(digest, duration)
		if err != nil {
			return fmt.Errorf("failed to pack allowlist tx: %w", err)
		}
		return h.LogMSIGNextSteps(txData, digest, bundlePath)
	}

	// ---------------- EOA: allowlist (if needed) and POST ----------------
	ownerAddr := ethcommon.HexToAddress(owner)

	allowlisted, err := h.Wrc.IsRequestAllowlisted(ownerAddr, digest)
	if err != nil {
		return fmt.Errorf("allowlist check failed: %w", err)
	}

	if !allowlisted {
		if err := h.Wrc.AllowlistRequest(digest, duration); err != nil {
			return fmt.Errorf("allowlist request failed: %w", err)
		}
		fmt.Printf("Digest allowlisted; proceeding to gateway POST: owner=%s, digest=0x%x\n", ownerAddr.Hex(), digest)
	} else {
		fmt.Printf("Digest already allowlisted; proceeding to gateway POST: owner=%s, digest=0x%x\n", ownerAddr.Hex(), digest)
	}

	// POST to gateway
	respBody, status, err := h.Gw.Post(body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("gateway returned a non-200 status code: %d", status)
	}

	// Parse/log results
	return h.ParseVaultGatewayResponse(vaulttypes.MethodSecretsList, respBody)
}

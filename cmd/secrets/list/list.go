package list

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/actions/vault"
	"github.com/smartcontractkit/chainlink-common/pkg/jsonrpc2"
	nautilus "github.com/smartcontractkit/chainlink-common/pkg/nodeauth/utils"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/dev-platform/cmd/secrets/common"
	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/runtime"
	"github.com/smartcontractkit/dev-platform/internal/settings"
)

// cre secrets list --namespace main --timeout 1h
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
			if duration < 0 || duration > constants.MaxVaultAllowlistDuration {
				ctx.Logger.Error().Dur("timeout", duration).Msg("invalid timeout: must be > 0 and < 168h (7d)")
				return fmt.Errorf("invalid --timeout: must be greater than 0 and less than 168h (7d)")
			}

			return Execute(h, namespace, duration, ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType)
		},
	}

	cmd.Flags().StringVar(&namespace, "namespace", "main", "Namespace to list (default: main)")
	settings.AddRawTxFlag(cmd)

	return cmd
}

// Execute performs: derive digest → (optional MSIG path) → allowlist digest → POST → parse.
func Execute(h *common.Handler, namespace string, duration time.Duration, ownerType string) error {
	owner := h.OwnerAddress
	if namespace == "" {
		namespace = "main"
	}

	seed := &vault.ListSecretIdentifiersRequest{
		Owner:     owner,
		Namespace: namespace,
	}
	digest := nautilus.CalculateRequestDigest(seed)

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

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
	}

	// MSIG path: provide next steps and exit before making a gateway call.
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

	// Allowlist digest on-chain (or skip if already allowlisted).
	wrV2Client, err := h.ClientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("create workflow registry client failed: %w", err)
	}
	ownerAddr := ethcommon.HexToAddress(owner)
	allowlisted, err := wrV2Client.IsRequestAllowlisted(ownerAddr, digest)
	if err != nil {
		return fmt.Errorf("allowlist check failed: %w", err)
	}

	if !allowlisted {
		if err := wrV2Client.AllowlistRequest(digest, duration); err != nil {
			return fmt.Errorf("allowlist request failed: %w", err)
		}
		h.Log.Info().Str("owner", ownerAddr.Hex()).Str("digest", digest).
			Msg("Digest allowlisted; proceeding to gateway POST")
	} else {
		h.Log.Info().Str("owner", ownerAddr.Hex()).Str("digest", digest).
			Msg("Digest already allowlisted; skipping on-chain allowlist")
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

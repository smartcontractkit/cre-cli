package execute

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// New creates the 'secrets execute' command that performs MSIG step 2 for any method.
// Usage: cre secrets execute <bundle.json>
func New(ctx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "execute [BUNDLE_PATH]",
		Short:   "Executes a previously prepared MSIG bundle (.json): verifies allowlist and POSTs the exact saved request.",
		Example: "cre secrets execute 157364...af4d5.json",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType != constants.WorkflowOwnerTypeMSIG {
				return fmt.Errorf("execute command is only supported for MSIG workflow owner type, add --unsigned flag")
			}

			bundlePath := args[0]

			ext := strings.ToLower(filepath.Ext(bundlePath))
			if ext != ".json" {
				return fmt.Errorf("execute expects a bundle .json file; got %q", ext)
			}

			h, err := common.NewHandler(ctx, bundlePath)
			if err != nil {
				return err
			}

			// Load the bundle, error if missing fields in json
			b, err := common.LoadBundle(bundlePath)
			if err != nil {
				return fmt.Errorf("failed to load bundle: %w", err)
			}

			// Validate method (supports create/update/delete/list)
			switch b.Method {
			case vaulttypes.MethodSecretsCreate,
				vaulttypes.MethodSecretsUpdate,
				vaulttypes.MethodSecretsDelete,
				vaulttypes.MethodSecretsList:
				// ok
			default:
				return fmt.Errorf("unsupported bundle method %q", b.Method)
			}

			// Check allowlist on-chain using the digest from bundle
			digest, err := common.HexToBytes32(b.DigestHex)
			if err != nil {
				return fmt.Errorf("invalid bundle digest: %w", err)
			}

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
				return fmt.Errorf("on-chain request (digest %s) is not finalized/allowlisted. Finalize the allowlist tx, then rerun this command", b.DigestHex)
			}

			// POST the exact saved body
			respBody, status, err := h.Gw.Post(b.RequestBody)
			if err != nil {
				return err
			}
			if status != http.StatusOK {
				return fmt.Errorf("gateway returned a non-200 status code: %d", status)
			}

			// Parse & print results according to the bundle method
			return h.ParseVaultGatewayResponse(b.Method, respBody)
		},
	}

	settings.AddRawTxFlag(cmd)

	return cmd
}

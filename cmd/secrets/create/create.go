package create

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// New creates and returns the 'secrets create' cobra command.
func New(ctx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create [SECRETS_FILE_PATH]",
		Short:   "Creates secrets from a YAML file provided as a positional argument.",
		Example: "cre secrets create my-secrets.yaml",
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

			reqIDFlag, err := cmd.Flags().GetString("request-id")
			if err != nil {
				return err
			}

			inputs, err := h.ResolveInputs()
			if err != nil {
				return err
			}
			if err := h.ValidateInputs(inputs); err != nil {
				return err
			}

			return h.Execute(inputs, vaulttypes.MethodSecretsCreate, duration, ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType, reqIDFlag)
		},
	}

	settings.AddRawTxFlag(cmd)
	cmd.Flags().String("request-id", "", "Reuse a specific request ID (UUID). When provided, the command will not create a new ID and will only proceed if the corresponding on-chain allowlist entry is already finalized.")

	return cmd
}

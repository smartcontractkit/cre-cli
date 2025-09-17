package create

import (
	"fmt"

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
		Short:   "Creates secrets from a JSON file provided as a positional argument.",
		Example: "cre secrets create my-secrets.json",
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
			if duration < 0 || duration > constants.MaxVaultAllowlistDuration {
				ctx.Logger.Error().Dur("timeout", duration).Msg("invalid timeout: must be > 0 and < 168h (7d)")
				return fmt.Errorf("invalid --timeout: must be greater than 0 and less than 168h (7d)")
			}

			inputs, err := h.ResolveInputs()
			if err != nil {
				return err
			}

			if err := h.ValidateInputs(inputs); err != nil {
				return err
			}

			return h.Execute(inputs, vaulttypes.MethodSecretsCreate, duration, ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType)
		},
	}

	settings.AddRawTxFlag(cmd)

	return cmd
}

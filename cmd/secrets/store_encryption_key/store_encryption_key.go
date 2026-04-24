package store_encryption_key

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/chainlink/v2/core/capabilities/vault/vaulttypes"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/common"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// New creates and returns the 'secrets store-encryption-key' cobra command.
func New(ctx *runtime.Context) *cobra.Command {
	var passphrase string

	cmd := &cobra.Command{
		Use:   "store-encryption-key",
		Short: "Derives an AES-256 encryption key from a passphrase and stores it in VaultDON.",
		Long: `Derives a 32-byte AES-256 key from the given passphrase using HKDF-SHA256,
then stores it in VaultDON under the name "san_marino_aes_gcm_encryption_key".

This key is used by the confidential-http capability to encrypt response bodies
when EncryptOutput is set to true.`,
		Example: `  cre secrets store-encryption-key --passphrase "my-secret-passphrase"
  cre secrets store-encryption-key --passphrase "my-secret-passphrase" --unsigned`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if passphrase == "" {
				return fmt.Errorf("--passphrase is required and must not be empty")
			}

			key, err := common.DeriveEncryptionKey(passphrase)
			if err != nil {
				return fmt.Errorf("failed to derive encryption key: %w", err)
			}

			// Build a single-entry secrets payload with the derived key as a hex-encoded value.
			inputs := common.UpsertSecretsInputs{
				{
					ID:        common.EncryptionKeySecretName,
					Value:     hex.EncodeToString(key),
					Namespace: "main",
				},
			}

			// The handler needs a secrets file path for bundle naming. Use a
			// synthetic path so bundles land in the current directory.
			h, err := common.NewHandler(ctx, "encryption-key-secrets.yaml")
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
				return fmt.Errorf("invalid --timeout: must be greater than 0 and less than %dh (%dd)", maxHours, maxDays)
			}

			return h.Execute(inputs, vaulttypes.MethodSecretsCreate, duration,
				ctx.Settings.Workflow.UserWorkflowSettings.WorkflowOwnerType)
		},
	}

	cmd.Flags().StringVar(&passphrase, "passphrase", "", "Passphrase used to derive the AES-256 encryption key (required)")
	_ = cmd.MarkFlagRequired("passphrase")

	settings.AddTxnTypeFlags(cmd)
	settings.AddSkipConfirmation(cmd)
	return cmd
}

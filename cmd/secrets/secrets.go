package secrets

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/secrets/create"
	"github.com/smartcontractkit/cre-cli/cmd/secrets/decrypt_output"
	"github.com/smartcontractkit/cre-cli/cmd/secrets/delete"
	"github.com/smartcontractkit/cre-cli/cmd/secrets/execute"
	"github.com/smartcontractkit/cre-cli/cmd/secrets/list"
	"github.com/smartcontractkit/cre-cli/cmd/secrets/store_encryption_key"
	"github.com/smartcontractkit/cre-cli/cmd/secrets/update"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	var timeout time.Duration

	var secretsCmd = &cobra.Command{
		Use:    "secrets",
		Short:  "Handles secrets management",
		Hidden: false,
		Long:   `Create, update, delete, list secrets in Vault DON.`,
	}

	// Persistent flag available to all subcommands.
	secretsCmd.PersistentFlags().DurationVar(
		&timeout,
		"timeout",
		constants.DefaultVaultAllowlistDuration, // default 2 days
		"Timeout for secrets operations (e.g. 30m, 2h, 48h).",
	)

	secretsCmd.AddCommand(create.New(runtimeContext))
	secretsCmd.AddCommand(update.New(runtimeContext))
	secretsCmd.AddCommand(delete.New(runtimeContext))
	secretsCmd.AddCommand(list.New(runtimeContext))
	secretsCmd.AddCommand(execute.New(runtimeContext))
	secretsCmd.AddCommand(store_encryption_key.New(runtimeContext))
	secretsCmd.AddCommand(decrypt_output.New(runtimeContext))

	return secretsCmd
}

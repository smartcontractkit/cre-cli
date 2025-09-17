package secrets

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/dev-platform/cmd/secrets/create"
	"github.com/smartcontractkit/dev-platform/cmd/secrets/delete"
	"github.com/smartcontractkit/dev-platform/cmd/secrets/list"
	"github.com/smartcontractkit/dev-platform/cmd/secrets/update"
	"github.com/smartcontractkit/dev-platform/internal/constants"
	"github.com/smartcontractkit/dev-platform/internal/runtime"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	var timeout time.Duration

	var secretsCmd = &cobra.Command{
		Use:    "secrets",
		Short:  "Handles secrets management",
		Hidden: true,
		Long:   `Create, update, delete secrets in Vault DON.`,
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

	return secretsCmd
}

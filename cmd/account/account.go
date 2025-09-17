package account

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/dev-platform/cmd/account/link_key"
	"github.com/smartcontractkit/dev-platform/cmd/account/list_key"
	"github.com/smartcontractkit/dev-platform/cmd/account/unlink_key"
	"github.com/smartcontractkit/dev-platform/internal/runtime"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	accountCmd := &cobra.Command{
		Use:   "account",
		Short: "Manages account",
		Long:  "Manage your linked public key addresses for workflow operations.",
	}

	accountCmd.AddCommand(link_key.New(runtimeContext))
	accountCmd.AddCommand(unlink_key.New(runtimeContext))
	accountCmd.AddCommand(list_key.New(runtimeContext))

	return accountCmd
}

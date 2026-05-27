package account

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/account/access"
	"github.com/smartcontractkit/cre-cli/cmd/account/link_key"
	"github.com/smartcontractkit/cre-cli/cmd/account/list_key"
	"github.com/smartcontractkit/cre-cli/cmd/account/unlink_key"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	accountCmd := &cobra.Command{
		Use:   "account",
		Short: "Manage account and request deploy access",
		Long:  "Manage your linked public key addresses for workflow operations and request deployment access.",
	}

	accountCmd.AddCommand(access.New(runtimeContext))
	accountCmd.AddCommand(link_key.New(runtimeContext))
	accountCmd.AddCommand(unlink_key.New(runtimeContext))
	accountCmd.AddCommand(list_key.New(runtimeContext))

	return accountCmd
}

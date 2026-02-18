package contract

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/contract/deploy"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	contractCmd := &cobra.Command{
		Use:   "contract",
		Short: "Manages smart contracts",
		Long:  `The contract command allows you to deploy and manage smart contracts at the project level.`,
	}

	contractCmd.AddCommand(deploy.New(runtimeContext))

	return contractCmd
}

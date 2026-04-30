package supported_chains

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate/chain/evm"
)

func New() *cobra.Command {
	return &cobra.Command{
		Use:   "supported-chains",
		Short: "List all supported chain names",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			names := evm.SupportedChainNames()
			fmt.Println("Supported chain names:")
			for _, name := range names {
				fmt.Printf("  %s\n", name)
			}
			return nil
		},
	}
}

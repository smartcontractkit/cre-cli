package generatebindings

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/evm"
	"github.com/smartcontractkit/cre-cli/cmd/generate-bindings/solana"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	generateBindingsCmd := &cobra.Command{
		Use:   "generate-bindings",
		Short: "Generate bindings for contracts",
		Long:  `The generate-bindings command allows you to generate bindings for contracts.`,
	}

	generateBindingsCmd.AddCommand(evm.New(runtimeContext))
	generateBindingsCmd.AddCommand(solana.New(runtimeContext))

	return generateBindingsCmd
}

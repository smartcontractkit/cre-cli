package execution

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

// New returns the `execution` subcommand group wired under `cre workflow`.
func New(runtimeContext *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execution",
		Short: "Query workflow execution history",
		Long:  `The execution command provides visibility into workflow executions, node events, and logs.`,
	}

	cmd.AddCommand(newList(runtimeContext))
	cmd.AddCommand(newStatus(runtimeContext))
	cmd.AddCommand(newEvents(runtimeContext))
	cmd.AddCommand(newLogs(runtimeContext))

	return cmd
}

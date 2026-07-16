package execution

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/execution/events"
	"github.com/smartcontractkit/cre-cli/cmd/execution/list"
	"github.com/smartcontractkit/cre-cli/cmd/execution/logs"
	"github.com/smartcontractkit/cre-cli/cmd/execution/status"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

// New returns the top-level `execution` command group wired under `cre`.
func New(runtimeContext *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "execution",
		Short: "Query workflow execution history",
		Long:  `The execution command provides visibility into workflow executions, node events, and logs.`,
	}

	cmd.AddCommand(list.New(runtimeContext))
	cmd.AddCommand(status.New(runtimeContext))
	cmd.AddCommand(events.New(runtimeContext))
	cmd.AddCommand(logs.New(runtimeContext))

	return cmd
}

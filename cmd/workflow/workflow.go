package workflow

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/activate"
	"github.com/smartcontractkit/cre-cli/cmd/workflow/delete"
	"github.com/smartcontractkit/cre-cli/cmd/workflow/deploy"
	"github.com/smartcontractkit/cre-cli/cmd/workflow/id"
	"github.com/smartcontractkit/cre-cli/cmd/workflow/pause"
	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate"
	"github.com/smartcontractkit/cre-cli/cmd/workflow/test"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	workflowCmd := &cobra.Command{
		Use:   "workflow",
		Short: "Manages workflows",
		Long:  `The workflow command allows you to register and manage existing workflows.`,
	}

	workflowCmd.AddCommand(activate.New(runtimeContext))
	workflowCmd.AddCommand(delete.New(runtimeContext))
	workflowCmd.AddCommand(pause.New(runtimeContext))
	workflowCmd.AddCommand(test.New(runtimeContext))
	workflowCmd.AddCommand(deploy.New(runtimeContext))
	workflowCmd.AddCommand(simulate.New(runtimeContext))
	workflowCmd.AddCommand(id.New(runtimeContext))

	return workflowCmd
}

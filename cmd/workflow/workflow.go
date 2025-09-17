package workflow

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/dev-platform/cmd/workflow/activate"
	"github.com/smartcontractkit/dev-platform/cmd/workflow/delete"
	"github.com/smartcontractkit/dev-platform/cmd/workflow/deploy"
	"github.com/smartcontractkit/dev-platform/cmd/workflow/pause"
	"github.com/smartcontractkit/dev-platform/cmd/workflow/simulate"
	"github.com/smartcontractkit/dev-platform/cmd/workflow/test"
	"github.com/smartcontractkit/dev-platform/internal/runtime"
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

	return workflowCmd
}

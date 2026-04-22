package registry

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/registry/list"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/runtimeattach"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	registryCmd := &cobra.Command{
		Use:   "registry",
		Short: "Manages workflow registries",
		Long:  `The registry command lets you view and inspect the workflow registries available for your organization.`,
	}

	registryCmd.AddCommand(list.New(runtimeContext))

	runtimeattach.Register(registryCmd, runtimeattach.Empty)
	return registryCmd
}

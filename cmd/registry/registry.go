package registry

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/registry/list"
	"github.com/smartcontractkit/cre-cli/cmd/registry/quotas"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	registryCmd := &cobra.Command{
		Use:   "registry",
		Short: "Manages workflow registries",
		Long:  `The registry command lets you view and inspect the workflow registries available for your organization, and your usage against their quotas.`,
	}

	registryCmd.AddCommand(list.New(runtimeContext))
	registryCmd.AddCommand(quotas.New(runtimeContext))

	return registryCmd
}

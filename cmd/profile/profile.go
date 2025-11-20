package profile

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/profile/delete"
	"github.com/smartcontractkit/cre-cli/cmd/profile/list"
	"github.com/smartcontractkit/cre-cli/cmd/profile/rename"
	"github.com/smartcontractkit/cre-cli/cmd/profile/use"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	profileCmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage authentication profiles",
		Long:  "Manage multiple authentication profiles for different organizations and accounts.",
	}

	profileCmd.AddCommand(list.New(runtimeContext))
	profileCmd.AddCommand(use.New(runtimeContext))
	profileCmd.AddCommand(delete.New(runtimeContext))
	profileCmd.AddCommand(rename.New(runtimeContext))

	return profileCmd
}

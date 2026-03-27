package templates

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/templates/add"
	"github.com/smartcontractkit/cre-cli/cmd/templates/list"
	"github.com/smartcontractkit/cre-cli/cmd/templates/remove"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	templatesCmd := &cobra.Command{
		Use:   "templates",
		Short: "Manages template repository sources",
		Long: `Manages the template repository sources that cre init uses to discover templates.

cre init ships with a default set of templates ready to use.
Use these commands only if you want to add custom or third-party template repositories.

To scaffold a new project from a template, use: cre init`,
	}

	templatesCmd.AddCommand(list.New(runtimeContext))
	templatesCmd.AddCommand(add.New(runtimeContext))
	templatesCmd.AddCommand(remove.New(runtimeContext))

	return templatesCmd
}

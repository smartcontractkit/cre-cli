package list

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Lists available workflow registries for the current environment",
		Long: `Displays the registries configured for your organization, including type and address.

The ID shown for each registry is the value you set in workflow.yaml
under the ` + "`deployment-registry`" + ` key to target that registry, e.g.:

  <target-name>:
    user-workflow:
      workflow-name: "my-workflow"
      deployment-registry: "private"`,
		Example: `cre registry list`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtimeContext.TenantContext == nil {
				return fmt.Errorf("user context not available — run `cre login` and retry")
			}

			registries := runtimeContext.TenantContext.Registries
			if len(registries) == 0 {
				ui.Warning("No registries found for this environment")
				return nil
			}

			ui.Line()
			ui.Bold("Registries available to your organization")
			ui.Line()

			for _, r := range registries {
				ui.Bold(r.Label)
				ui.Dim(fmt.Sprintf("ID:   %s", r.ID))
				ui.Dim(fmt.Sprintf("Type: %s", r.Type))
				if r.Address != nil && *r.Address != "" {
					ui.Dim(fmt.Sprintf("Addr: %s", *r.Address))
				}
				ui.Line()
			}

			ui.Bold("Targeting a registry")
			ui.Dim("Set `deployment-registry` in your workflow.yaml to one of the IDs above:")
			ui.Line()
			ui.Code("  <target-name>:")
			ui.Code("    user-workflow:")
			ui.Code(fmt.Sprintf("      deployment-registry: %q", registries[0].ID))
			ui.Line()

			return nil
		},
	}
}

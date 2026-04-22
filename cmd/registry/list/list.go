package list

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/runtimeattach"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "Lists available workflow registries for the current environment",
		Long:    `Displays the registries configured for your organization, including type and address.`,
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

			return nil
		},
	}
	runtimeattach.Register(cmd, runtimeattach.CredsAndTenant)
	return cmd
}

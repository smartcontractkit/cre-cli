package limits

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate"
	"github.com/smartcontractkit/cre-cli/internal/runtimeattach"
)

func New() *cobra.Command {
	limitsCmd := &cobra.Command{
		Use:   "limits",
		Short: "Manage simulation limits",
		Long:  `The limits command provides tools for managing workflow simulation limits.`,
	}

	export := newExportCmd()
	limitsCmd.AddCommand(export)

	runtimeattach.Register(limitsCmd, runtimeattach.Empty)
	runtimeattach.Register(export, runtimeattach.Empty)
	return limitsCmd
}

func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export default simulation limits as JSON",
		Long: `Exports the default production simulation limits as JSON.
The output can be redirected to a file and customized.`,
		Example: `cre workflow limits export > my-limits.json`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data := simulate.ExportDefaultLimitsJSON()
			fmt.Println(string(data))
			return nil
		},
	}
}

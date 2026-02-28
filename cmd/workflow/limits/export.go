package limits

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/simulate"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func New(runtimeContext *runtime.Context) *cobra.Command {
	limitsCmd := &cobra.Command{
		Use:   "limits",
		Short: "Manage simulation limits",
		Long:  `The limits command provides tools for managing workflow simulation limits.`,
	}

	limitsCmd.AddCommand(newExportCmd())

	return limitsCmd
}

func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export default simulation limits as JSON",
		Long: `Exports the default production simulation limits as JSON.

The output can be redirected to a file and customized for use with
the --limits flag of the simulate command.

Example:
  cre workflow limits export > my-limits.json
  cre workflow simulate ./my-workflow --limits ./my-limits.json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			data := simulate.ExportDefaultLimitsJSON()
			fmt.Println(string(data))
			return nil
		},
	}
}

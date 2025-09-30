package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

// Default placeholder value
var Version = "development"

func New(runtimeContext *runtime.Context) *cobra.Command {
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the cre version",
		Long:  "This command prints the current version of the cre",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("cre", Version)
			return nil
		},
	}

	return versionCmd
}

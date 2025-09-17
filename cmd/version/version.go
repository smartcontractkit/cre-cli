package version

import (
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
			log := runtimeContext.Logger
			log.Info().Msgf("cre %s", Version)
			return nil
		},
	}

	return versionCmd
}

package gist

import (
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/upload/gist/batch"
	"github.com/smartcontractkit/cre-cli/cmd/upload/gist/single"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

// TDDO: Remove upload gist
func New(runtimeContext *runtime.Context) *cobra.Command {
	var gistCmd = &cobra.Command{
		Use:    "gist",
		Short:  "Upload files to a Github Gist",
		Hidden: true, // Hide this command from the help output, unhide after M2 release
		Long:   `The gist command allows you to upload one or more files to a new or existing Gist`,
	}

	gistCmd.AddCommand(single.New(runtimeContext))
	gistCmd.AddCommand(batch.New(runtimeContext))
	return gistCmd
}

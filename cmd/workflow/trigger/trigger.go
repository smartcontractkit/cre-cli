package trigger

import (
	"github.com/spf13/cobra"

	httptrigger "github.com/smartcontractkit/cre-cli/cmd/workflow/trigger/http"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

// New returns the hidden `trigger` command group wired under `cre workflow`.
func New(runtimeContext *runtime.Context) *cobra.Command {
	triggerCmd := &cobra.Command{
		Use:    "trigger",
		Short:  "Fires the triggers of deployed workflows",
		Long:   `The trigger command fires a deployed workflow's trigger through the CRE Gateway.`,
		Hidden: true, // Internal tooling for live E2E suites; not part of the public CLI surface.
	}

	triggerCmd.AddCommand(httptrigger.New(runtimeContext))

	return triggerCmd
}

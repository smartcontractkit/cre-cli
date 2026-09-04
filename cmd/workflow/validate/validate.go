package validate

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// Inputs captures the resolved inputs for the validate command.
//
// This command is a design mock: it illustrates the intended UX of
// "cre workflow validate" without performing a real reproducible build or
// writing to any registry. The real implementation would (1) read the source
// (public URL or local path), (2) rebuild the binary + config in a
// deterministic environment, (3) recompute the workflow hash, (4) compare it
// against the on-chain workflowID of the latest deployment, and (5) mark the
// deployment verified on a match.
type Inputs struct {
	SourceCode  string
	WorkflowID  string
	NoPublish   bool
	SkipConfirm bool
}

func New(_ *runtime.Context) *cobra.Command {
	validateCmd := &cobra.Command{
		Use:   "validate <workflow-folder-path>",
		Short: "Validate workflow source against its latest deployment",
		Long: `Verifies that a workflow's source code reproduces the binary and config
deployed on-chain, and marks the deployment verified when it matches.

The source may be a public URL or a local path shared with you out-of-band by
the workflow owner (for private code). The command reads the source, rebuilds
the workflow in a deterministic environment, recomputes the workflow hash, and
compares it to the workflowID of the latest deployment. On a match the
deployment is marked verified in CRE.`,
		Args: cobra.ExactArgs(1),
		Example: `  cre workflow validate ./my-workflow --source-code https://github.com/acme/workflow/releases/tag/v1.2.0
  cre workflow validate ./my-workflow --source-code ./shared-source/acme-workflow
  cre workflow validate ./my-workflow --source-code https://github.com/acme/workflow/tree/main --workflow-id 0x00986c...78038b
  cre workflow validate ./my-workflow --source-code https://github.com/acme/workflow/releases/tag/v1.2.0 --no-publish`,
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs := Inputs{
				SourceCode:  mustFlagString(cmd, "source-code"),
				WorkflowID:  mustFlagString(cmd, "workflow-id"),
				NoPublish:   mustFlagBool(cmd, "no-publish"),
				SkipConfirm: mustFlagBool(cmd, "yes"),
			}

			if inputs.SourceCode == "" {
				ui.Error("The --source-code flag is required")
				ui.ErrorWithHelp("No source provided", "Pass --source-code <url-or-path> pointing at the workflow source (public URL or local path).")
				return nil
			}

			return Execute(cmd.Context(), inputs, args[0])
		},
	}

	validateCmd.Flags().String("source-code", "", "Path or URL to the workflow source (public URL, or local path to privately shared code)")
	validateCmd.Flags().String("workflow-id", "", "Workflow ID of the deployment to validate against (defaults to the latest deployment)")
	validateCmd.Flags().Bool("no-publish", false, "Validate only; do not update the published code URL")
	validateCmd.Flags().Bool("yes", false, "Skip the confirmation prompt before publishing the code URL")

	return validateCmd
}

// Execute runs the mock validation flow and prints its intended output.
func Execute(ctx context.Context, inputs Inputs, workflowPath string) error {
	ui.Title("Validate workflow source")

	ui.Dim("Workflow folder: " + workflowPath)
	if inputs.WorkflowID != "" {
		ui.Dim("Target deployment: " + inputs.WorkflowID)
	} else {
		ui.Dim("Target deployment: latest")
	}

	ui.Line()

	// 1. Read source
	ui.Step("1. Reading source code")
	ui.Code("   " + inputs.SourceCode)
	ui.Success("Source loaded")

	// 2. Reproducible build
	ui.Step("2. Reproducible build (deterministic environment)")
	spinner := ui.NewSpinner()
	spinner.Start("Compiling workflow…")
	time.Sleep(600 * time.Millisecond)
	spinner.Stop()
	ui.Success("Binary + config rebuilt")

	// 3. Hash computation
	ui.Step("3. Computing workflow hash")
	ui.Dim("Binary hash:     0x3fd0a1c2b4e5f60718293a4b5c6d7e8f9")
	ui.Dim("Config hash:     0x9a21c7d0e4f5a1b2c3d4e5f60718293a4")
	ui.Dim("Workflow hash:   0x00986cf3a1b2c4d5e6f708192a3b4c5d")

	// 4. Compare against on-chain workflowID
	ui.Step("4. Comparing to on-chain deployment")
	ui.Code("   on-chain: 0x00986cf3a1b2c4d5e6f708192a3b4c5d")
	ui.Code("   computed: 0x00986cf3a1b2c4d5e6f708192a3b4c5d")
	ui.Success("Hash matches latest deployment")

	ui.Line()

	if inputs.NoPublish {
		ui.Success("Validation complete — nothing has been published")
		return nil
	}

	ui.Step("Publishing verified code URL")
	ui.URL("   " + inputs.SourceCode)
	ui.Success("Deployment marked verified in CRE")

	ui.Line()
	ui.Success("Workflow is now verifiable in CRE Explorer (status: Verified)")

	return nil
}

func mustFlagString(cmd *cobra.Command, name string) string {
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		return ""
	}
	return v
}

func mustFlagBool(cmd *cobra.Command, name string) bool {
	v, err := cmd.Flags().GetBool(name)
	if err != nil {
		return false
	}
	return v
}

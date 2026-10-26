package delete

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	workflowcommon "github.com/smartcontractkit/cre-cli/cmd/workflow/common"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

type Inputs struct {
	WorkflowName     string `validate:"workflow_name"`
	WorkflowOwner    string `validate:"workflow_owner"`
	WorkflowID       string
	RPCURL           string
	SkipConfirmation bool
	NonInteractive   bool
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:   "delete [workflow-name-or-id | workflow-folder-path]",
		Short: "Deletes all versions of a workflow from the Workflow Registry",
		Long: `Deletes all workflow versions matching the given name and owner address.

The argument can be a workflow folder path (project-based flow), a workflow
name, or an on-chain workflow ID. When the argument is a name or ID — or is
omitted for an interactive picker — no project folder is needed; the workflow
is resolved from the CRE platform. On-chain workflows additionally require
--rpc-url and CRE_ETH_PRIVATE_KEY in that mode.`,
		Args: cobra.MaximumNArgs(1),
		Example: `cre workflow delete ./my-workflow
  cre workflow delete my-workflow
  cre workflow delete 00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856
  cre workflow delete my-workflow --rpc-url https://...
  cre workflow delete   # interactive picker`,
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext, cmd.InOrStdin())

			arg := ""
			if len(args) == 1 {
				arg = args[0]
			}

			inputs, err := handler.ResolveInputs(cmd.Context(), runtimeContext.Viper, arg)
			if err != nil {
				return err
			}
			handler.inputs = inputs
			err = handler.ValidateInputs()
			if err != nil {
				return err
			}
			return handler.Execute(cmd.Context())
		},
	}

	settings.AddTxnTypeFlags(deleteCmd)
	settings.AddSkipConfirmation(deleteCmd)
	settings.AddFolderlessLifecycleFlags(deleteCmd)

	return deleteCmd
}

type handler struct {
	log            *zerolog.Logger
	clientFactory  client.Factory
	v              *viper.Viper
	stdin          io.Reader
	settings       *settings.Settings
	credentials    *credentials.Credentials
	environmentSet *environments.EnvironmentSet
	inputs         Inputs
	runtimeContext *runtime.Context

	// folderlessRow is the platform workflow row when running without a project
	// folder; nil in the legacy project-based flow.
	folderlessRow *workflowresolve.Workflow

	validated bool
	execCtx   context.Context
}

func newHandler(ctx *runtime.Context, stdin io.Reader) *handler {
	h := handler{
		log:            ctx.Logger,
		clientFactory:  ctx.ClientFactory,
		v:              ctx.Viper,
		stdin:          stdin,
		settings:       ctx.Settings,
		credentials:    ctx.Credentials,
		environmentSet: ctx.EnvironmentSet,
		runtimeContext: ctx,
		validated:      false,
	}

	return &h
}

func (h *handler) ResolveInputs(ctx context.Context, v *viper.Viper, arg string) (Inputs, error) {
	inputs := Inputs{
		SkipConfirmation: v.GetBool(settings.Flags.SkipConfirmation.Name),
		NonInteractive:   v.GetBool(settings.Flags.NonInteractive.Name),
		RPCURL:           strings.TrimSpace(v.GetString(settings.Flags.RPCURL.Name)),
	}

	// Folder-less mode: settings were not loaded; resolve everything from the platform.
	if h.runtimeContext.Settings == nil {
		return h.resolveFolderlessInputs(ctx, v, arg, inputs)
	}

	resolvedWorkflowOwner, err := h.resolveWorkflowOwner()
	if err != nil {
		return Inputs{}, fmt.Errorf("failed to resolve workflow owner: %w", err)
	}

	inputs.WorkflowName = h.settings.Workflow.UserWorkflowSettings.WorkflowName
	inputs.WorkflowOwner = resolvedWorkflowOwner
	return inputs, nil
}

func (h *handler) resolveFolderlessInputs(ctx context.Context, v *viper.Viper, arg string, inputs Inputs) (Inputs, error) {
	if err := workflowcommon.GuardFolderlessTxnType(v); err != nil {
		return Inputs{}, err
	}

	target, err := workflowcommon.ResolveFolderlessTarget(ctx, h.runtimeContext, arg, workflowresolve.RowResolveOptions{
		OwnerFilter:    strings.TrimSpace(v.GetString(settings.Flags.Owner.Name)),
		RegistryFilter: strings.TrimSpace(v.GetString(settings.Flags.Registry.Name)),
		NonInteractive: inputs.NonInteractive,
		PickerTitle:    "Select a workflow to delete",
	})
	if err != nil {
		return Inputs{}, err
	}

	h.runtimeContext.ResolvedRegistry = target.Registry
	h.folderlessRow = &target.Workflow
	inputs.WorkflowName = target.Workflow.Name
	inputs.WorkflowOwner = target.Workflow.OwnerAddress
	inputs.WorkflowID = target.Workflow.WorkflowID

	if onChain, ok := target.Registry.(*settings.OnChainRegistry); ok {
		if err := workflowcommon.VerifyEOAOwner(v, target.Workflow.OwnerAddress); err != nil {
			return Inputs{}, err
		}
		if inputs.RPCURL == "" {
			return Inputs{}, fmt.Errorf(
				"missing RPC URL for chain %q: pass --%s to delete an on-chain workflow without a project folder",
				onChain.ChainName(), settings.Flags.RPCURL.Name,
			)
		}
	}

	return inputs, nil
}

// resolveWorkflowOwner returns the effective owner address for workflow ID computation.
// For private registry deploys, the derived workflow owner from the runtime context is used.
// For onchain deploys, the configured WorkflowOwner address is used directly.
func (h *handler) resolveWorkflowOwner() (string, error) {
	if h.runtimeContext.ResolvedRegistry.Type() != settings.RegistryTypeOffChain {
		return h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress, nil
	}

	owner := h.runtimeContext.DerivedWorkflowOwner
	if owner == "" {
		return "", fmt.Errorf("derived workflow owner is not available; ensure authentication succeeded")
	}

	return owner, nil
}

func (h *handler) ValidateInputs() error {
	validate, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to initialize validator: %w", err)
	}

	if err := validate.Struct(h.inputs); err != nil {
		return validate.ParseValidationErrors(err)
	}

	h.validated = true
	return nil
}

func (h *handler) Execute(ctx context.Context) error {
	if !h.validated {
		return fmt.Errorf("handler inputs not validated")
	}

	h.execCtx = ctx

	adapter, err := newRegistryDeleteStrategy(h.runtimeContext.ResolvedRegistry, h)
	if err != nil {
		return err
	}

	workflowcommon.DisplayWorkflowDetails(
		h.settings,
		h.runtimeContext,
		"Deleting",
		h.inputs.WorkflowName,
		h.inputs.WorkflowOwner,
	)

	workflows, err := adapter.FetchWorkflows()
	if err != nil {
		return err
	}

	if len(workflows) == 0 {
		ui.Warning(fmt.Sprintf("No workflows found for name: %s", h.inputs.WorkflowName))
		return nil
	}

	// Note: The way deploy is set up, there will only ever be one workflow in the command for now
	h.runtimeContext.Workflow.ID = workflows[0].ID

	ui.Bold(fmt.Sprintf("Found %d workflow(s) to delete for name: %s", len(workflows), h.inputs.WorkflowName))
	for i, wf := range workflows {
		ui.Print(fmt.Sprintf("   %d. Workflow", i+1))
		ui.Dim(fmt.Sprintf("      Registry:         %s", h.runtimeContext.ResolvedRegistry.ID()))
		ui.Dim(fmt.Sprintf("      ID:               %s", wf.ID))
		ui.Dim(fmt.Sprintf("      Owner:            %s", wf.Owner))
		ui.Dim(fmt.Sprintf("      DON Family:       %s", wf.DonFamily))
		ui.Dim(fmt.Sprintf("      Tag:              %s", wf.Tag))
		ui.Dim(fmt.Sprintf("      Binary URL:       %s", wf.BinaryURL))
		ui.Dim(fmt.Sprintf("      Workflow Status:  %s", wf.Status))
		ui.Line()
	}

	shouldDeleteWorkflow, err := h.shouldDeleteWorkflow(h.inputs.SkipConfirmation, h.inputs.WorkflowName)
	if err != nil {
		return err
	}
	if !shouldDeleteWorkflow {
		ui.Warning("Workflow deletion canceled")
		return nil
	}

	ui.Dim(fmt.Sprintf("Deleting %d workflow(s)...", len(workflows)))

	err = adapter.DeleteWorkflows(workflows)
	if err != nil {
		return err
	}

	ui.Success("Workflows deleted successfully")
	return nil
}

func (h *handler) shouldDeleteWorkflow(skipConfirmation bool, workflowName string) (bool, error) {
	if h.inputs.NonInteractive && !skipConfirmation {
		ui.ErrorWithSuggestions(
			"Non-interactive mode requires all inputs via flags",
			[]string{"--yes"},
		)
		return false, fmt.Errorf("missing required flags for --non-interactive mode")
	}
	if skipConfirmation {
		return true, nil
	}

	shouldDeleteWorkflow, err := h.askForWorkflowDeletionConfirmation(workflowName)
	if err != nil {
		return false, fmt.Errorf("failed to get workflow deletion confirmation: %w", err)
	}
	return shouldDeleteWorkflow, nil
}

func (h *handler) askForWorkflowDeletionConfirmation(expectedWorkflowName string) (bool, error) {
	ui.Warning(fmt.Sprintf("Are you sure you want to delete the workflow '%s'?", expectedWorkflowName))
	ui.Error("This action cannot be undone.")
	ui.Line()

	result, err := ui.Input(fmt.Sprintf("To confirm, type the workflow name: %s", expectedWorkflowName))
	if err != nil {
		return false, fmt.Errorf("failed to get workflow name confirmation: %w", err)
	}

	return result == expectedWorkflowName, nil
}

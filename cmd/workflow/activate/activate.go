package activate

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	workflowcommon "github.com/smartcontractkit/cre-cli/cmd/workflow/common"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

const (
	WorkflowStatusPaused = uint8(1)
)

type Inputs struct {
	WorkflowName     string `validate:"workflow_name"`
	WorkflowOwner    string `validate:"workflow_owner"`
	DonFamily        string `validate:"required"`
	WorkflowID       string
	RPCURL           string
	SkipConfirmation bool
	NonInteractive   bool
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	activateCmd := &cobra.Command{
		Use:   "activate [workflow-name-or-id | workflow-folder-path]",
		Short: "Activates workflow on the Workflow Registry contract",
		Long: `Changes workflow status to active on the Workflow Registry contract.

The argument can be a workflow folder path (project-based flow), a workflow
name, or an on-chain workflow ID. When the argument is a name or ID — or is
omitted for an interactive picker — no project folder is needed; the workflow
is resolved from the CRE platform. On-chain workflows additionally require
--rpc-url and CRE_ETH_PRIVATE_KEY in that mode.`,
		Args: cobra.MaximumNArgs(1),
		Example: `cre workflow activate ./my-workflow
  cre workflow activate my-workflow
  cre workflow activate 00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856
  cre workflow activate my-workflow --rpc-url https://... --yes
  cre workflow activate   # interactive picker`,
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext)

			arg := ""
			if len(args) == 1 {
				arg = args[0]
			}

			inputs, err := handler.ResolveInputs(cmd.Context(), runtimeContext.Viper, arg)
			if err != nil {
				return err
			}
			handler.inputs = inputs

			if err := handler.ValidateInputs(); err != nil {
				return err
			}
			return handler.Execute(cmd.Context())
		},
	}

	settings.AddTxnTypeFlags(activateCmd)
	settings.AddSkipConfirmation(activateCmd)
	settings.AddFolderlessLifecycleFlags(activateCmd)

	return activateCmd
}

type handler struct {
	log            *zerolog.Logger
	clientFactory  client.Factory
	settings       *settings.Settings
	environmentSet *environments.EnvironmentSet
	inputs         Inputs
	runtimeContext *runtime.Context

	// folderlessRow is the platform workflow row when running without a project
	// folder; nil in the legacy project-based flow.
	folderlessRow *workflowresolve.Workflow

	validated bool
	execCtx   context.Context
}

func newHandler(ctx *runtime.Context) *handler {
	h := handler{
		log:            ctx.Logger,
		clientFactory:  ctx.ClientFactory,
		settings:       ctx.Settings,
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

	resolvedWorkflowOwner, err := h.resolveWorkflowOwner(h.runtimeContext.ResolvedRegistry.Type())
	if err != nil {
		return Inputs{}, fmt.Errorf("failed to resolve workflow owner: %w", err)
	}

	inputs.WorkflowName = h.settings.Workflow.UserWorkflowSettings.WorkflowName
	inputs.WorkflowOwner = resolvedWorkflowOwner
	inputs.DonFamily = h.runtimeContext.ResolvedRegistry.DonFamily()
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
		CandidateFilter: func(w workflowresolve.Workflow) bool {
			return strings.EqualFold(w.Status, "PAUSED")
		},
		PickerTitle: "Select a workflow to activate",
	})
	if err != nil {
		return Inputs{}, err
	}

	h.runtimeContext.ResolvedRegistry = target.Registry
	h.folderlessRow = &target.Workflow
	inputs.WorkflowName = target.Workflow.Name
	inputs.WorkflowOwner = target.Workflow.OwnerAddress
	inputs.WorkflowID = target.Workflow.WorkflowID
	inputs.DonFamily = target.Registry.DonFamily()
	if inputs.DonFamily == "" {
		return Inputs{}, fmt.Errorf(
			"cannot determine the DON family for workflow %q; set %s or run from your project folder",
			target.Workflow.Name, environments.EnvVarDonFamily,
		)
	}

	if onChain, ok := target.Registry.(*settings.OnChainRegistry); ok {
		if err := workflowcommon.VerifyEOAOwner(v, target.Workflow.OwnerAddress); err != nil {
			return Inputs{}, err
		}
		if inputs.RPCURL == "" {
			return Inputs{}, fmt.Errorf(
				"missing RPC URL for chain %q: pass --%s to activate an on-chain workflow without a project folder",
				onChain.ChainName(), settings.Flags.RPCURL.Name,
			)
		}
	}

	return inputs, nil
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
	h.execCtx = ctx

	if !h.validated {
		return fmt.Errorf("handler inputs not validated")
	}

	if h.inputs.NonInteractive && !h.inputs.SkipConfirmation {
		ui.ErrorWithSuggestions(
			"Non-interactive mode requires all inputs via flags",
			[]string{"--yes"},
		)
		return fmt.Errorf("missing required flags for --non-interactive mode")
	}

	workflowcommon.DisplayWorkflowDetails(
		h.settings,
		h.runtimeContext,
		"Activating",
		h.inputs.WorkflowName,
		h.inputs.WorkflowOwner,
	)

	strategy, err := newRegistryActivateStrategy(h.runtimeContext.ResolvedRegistry, h)
	if err != nil {
		return err
	}

	return strategy.Activate()
}

// resolveWorkflowOwner returns the effective owner address for workflow ID computation.
// For private registry deploys, the derived workflow owner from the runtime context is used.
// For onchain deploys, the configured WorkflowOwner address is used directly.
func (h *handler) resolveWorkflowOwner(registryType settings.RegistryType) (string, error) {
	if registryType != settings.RegistryTypeOffChain {
		return h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress, nil
	}

	owner := h.runtimeContext.DerivedWorkflowOwner
	if owner == "" {
		return "", fmt.Errorf("derived workflow owner is not available; ensure authentication succeeded")
	}

	return owner, nil
}

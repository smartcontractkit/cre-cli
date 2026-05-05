package pause

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

const (
	WorkflowStatusActive = uint8(0)
)

type Inputs struct {
	WorkflowName     string `validate:"workflow_name"`
	WorkflowOwner    string `validate:"workflow_owner"`
	SkipConfirmation bool
	NonInteractive   bool
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var pauseCmd = &cobra.Command{
		Use:     "pause <workflow-folder-path>",
		Short:   "Pauses workflow on the Workflow Registry contract",
		Long:    `Changes workflow status to paused on the Workflow Registry contract`,
		Args:    cobra.ExactArgs(1),
		Example: `cre workflow pause ./my-workflow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext)

			inputs, err := handler.ResolveInputs(runtimeContext.Viper)
			if err != nil {
				return err
			}
			handler.inputs = inputs

			if err := handler.ValidateInputs(); err != nil {
				return err
			}
			return handler.Execute()
		},
	}

	settings.AddTxnTypeFlags(pauseCmd)
	settings.AddSkipConfirmation(pauseCmd)
	return pauseCmd
}

type handler struct {
	log            *zerolog.Logger
	clientFactory  client.Factory
	settings       *settings.Settings
	environmentSet *environments.EnvironmentSet
	inputs         Inputs
	runtimeContext *runtime.Context

	validated bool
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

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	resolvedWorkflowOwner, err := h.resolveWorkflowOwner(h.runtimeContext.ResolvedRegistry.Type())
	if err != nil {
		return Inputs{}, fmt.Errorf("failed to resolve workflow owner: %w", err)
	}

	return Inputs{
		WorkflowName:     h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner:    resolvedWorkflowOwner,
		SkipConfirmation: v.GetBool(settings.Flags.SkipConfirmation.Name),
		NonInteractive:   v.GetBool(settings.Flags.NonInteractive.Name),
	}, nil
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

func (h *handler) Execute() error {
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

	h.displayWorkflowDetails()

	strategy, err := newRegistryPauseStrategy(h.runtimeContext.ResolvedRegistry, h)
	if err != nil {
		return err
	}

	return strategy.Pause()
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

func (h *handler) displayWorkflowDetails() {
	ui.Line()
	ui.Title(fmt.Sprintf("Pausing Workflow: %s", h.inputs.WorkflowName))
	ui.Dim(fmt.Sprintf("Target:        %s", h.settings.User.TargetName))
	ui.Dim(fmt.Sprintf("Owner Address: %s", h.inputs.WorkflowOwner))
	ui.Line()
}

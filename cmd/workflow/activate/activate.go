package activate

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/runtimeattach"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

const (
	WorkflowStatusPaused = uint8(1)
)

type Inputs struct {
	WorkflowName  string `validate:"workflow_name"`
	WorkflowOwner string `validate:"workflow_owner"`
	DonFamily     string `validate:"required"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	activateCmd := &cobra.Command{
		Use:     "activate <workflow-folder-path>",
		Short:   "Activates workflow on the Workflow Registry contract",
		Long:    `Changes workflow status to active on the Workflow Registry contract`,
		Args:    cobra.ExactArgs(1),
		Example: `cre workflow activate ./my-workflow`,
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

	settings.AddTxnTypeFlags(activateCmd)
	settings.AddSkipConfirmation(activateCmd)

	runtimeattach.Register(activateCmd, runtimeattach.FullWithDeploymentRPC)
	return activateCmd
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
		WorkflowName:  h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner: resolvedWorkflowOwner,
		DonFamily:     h.runtimeContext.ResolvedRegistry.DonFamily(),
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

	h.displayWorkflowDetails()

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

func (h *handler) displayWorkflowDetails() {
	ui.Line()
	ui.Title(fmt.Sprintf("Activating Workflow: %s", h.inputs.WorkflowName))
	ui.Dim(fmt.Sprintf("Target:        %s", h.settings.User.TargetName))
	ui.Dim(fmt.Sprintf("Owner Address: %s", h.inputs.WorkflowOwner))
	ui.Line()
}

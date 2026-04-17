package delete

import (
	"fmt"
	"io"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	WorkflowName     string `validate:"workflow_name"`
	WorkflowOwner    string `validate:"workflow_owner"`
	SkipConfirmation bool
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:     "delete <workflow-folder-path>",
		Short:   "Deletes all versions of a workflow from the Workflow Registry",
		Long:    "Deletes all workflow versions matching the given name and owner address.",
		Args:    cobra.ExactArgs(1),
		Example: `cre workflow delete ./my-workflow`,
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext, cmd.InOrStdin())

			inputs, err := handler.ResolveInputs(runtimeContext.Viper)
			if err != nil {
				return err
			}
			handler.inputs = inputs
			err = handler.ValidateInputs()
			if err != nil {
				return err
			}
			return handler.Execute()
		},
	}

	settings.AddTxnTypeFlags(deleteCmd)
	settings.AddSkipConfirmation(deleteCmd)

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

	validated bool
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

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	resolvedWorkflowOwner, err := h.resolveWorkflowOwner()
	if err != nil {
		return Inputs{}, fmt.Errorf("failed to resolve workflow owner: %w", err)
	}

	return Inputs{
		WorkflowName:     h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner:    resolvedWorkflowOwner,
		SkipConfirmation: v.GetBool(settings.Flags.SkipConfirmation.Name),
	}, nil
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

	if len(owner) >= 2 && owner[:2] != "0x" {
		owner = "0x" + owner
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

func (h *handler) Execute() error {
	adapter, err := newRegistryDeleteStrategy(h.runtimeContext.ResolvedRegistry, h)
	if err != nil {
		return err
	}

	return adapter.Delete()
}

func (h *handler) shouldDeleteWorkflow(skipConfirmation bool, workflowName string) (bool, error) {
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

func (h *handler) displayWorkflowDetails() {
	ui.Line()
	ui.Title(fmt.Sprintf("Deleting Workflow: %s", h.inputs.WorkflowName))
	ui.Dim(fmt.Sprintf("Target:        %s", h.settings.User.TargetName))
	ui.Dim(fmt.Sprintf("Owner Address: %s", h.inputs.WorkflowOwner))
	ui.Line()
}

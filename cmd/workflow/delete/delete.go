package delete

import (
	"encoding/hex"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/prompt"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	WorkflowName     string `validate:"workflow_name"`
	WorkflowOwner    string `validate:"workflow_owner"`
	SkipConfirmation bool

	WorkflowRegistryContractAddress   string `validate:"required"`
	WorkflowRegistryContractChainName string `validate:"required"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:   "delete <workflow-name>",
		Short: "Deletes all versions of a workflow from the Workflow Registry",
		Long:  "Deletes all workflow versions matching the given name and owner address.",
		Args:  cobra.ExactArgs(1),
		Example: `
		cre workflow delete my-workflow
		`,
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

	settings.AddRawTxFlag(deleteCmd)
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
	validated      bool
}

func newHandler(ctx *runtime.Context, stdin io.Reader) *handler {
	return &handler{
		log:            ctx.Logger,
		clientFactory:  ctx.ClientFactory,
		v:              ctx.Viper,
		stdin:          stdin,
		settings:       ctx.Settings,
		credentials:    ctx.Credentials,
		environmentSet: ctx.EnvironmentSet,
		validated:      false,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	return Inputs{
		WorkflowName:                      h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner:                     h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		SkipConfirmation:                  v.GetBool(settings.Flags.SkipConfirmation.Name),
		WorkflowRegistryContractChainName: h.environmentSet.WorkflowRegistryChainName,
		WorkflowRegistryContractAddress:   h.environmentSet.WorkflowRegistryAddress,
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
	workflowName := h.inputs.WorkflowName
	workflowOwner := common.HexToAddress(h.inputs.WorkflowOwner)

	wrc, err := h.clientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("failed to create workflow registry client: %w", err)
	}

	allWorkflows, err := wrc.GetWorkflowListByOwnerAndName(workflowOwner, workflowName, big.NewInt(0), big.NewInt(100))
	if err != nil {
		return fmt.Errorf("failed to get workflow list: %w", err)
	}
	if len(allWorkflows) == 0 {
		h.log.Info().Msgf("No workflows found for name: %s", workflowName)
		return nil
	}

	h.log.Info().Msgf("Found %d workflow(s) to delete for name: %s", len(allWorkflows), workflowName)
	for i, wf := range allWorkflows {
		status := map[uint8]string{0: "ACTIVE", 1: "PAUSED"}[wf.Status]
		h.log.Info().Msgf("   %d. Workflow", i+1)
		h.log.Info().Msgf("      ID:              %s", hex.EncodeToString(wf.WorkflowId[:]))
		h.log.Info().Msgf("      Owner:           %s", wf.Owner.Hex())
		h.log.Info().Msgf("      DON Family:      %s", wf.DonFamily)
		h.log.Info().Msgf("      Tag:             %s", wf.Tag)
		h.log.Info().Msgf("      Binary URL:      %s", wf.BinaryUrl)
		h.log.Info().Msgf("      Workflow Status: %s", status)
		h.log.Info().Msg("")
	}

	shouldDeleteWorkflow, err := h.shouldDeleteWorkflow(h.inputs.SkipConfirmation, workflowName)
	if err != nil {
		return err
	}
	if !shouldDeleteWorkflow {
		h.log.Info().Msg("Workflow deletion canceled")
		return nil
	}

	h.log.Info().Msgf("Deleting %d workflow(s)...", len(allWorkflows))
	for _, wf := range allWorkflows {
		txOut, err := wrc.DeleteWorkflow(wf.WorkflowId)
		if err != nil {
			h.log.Error().
				Err(err).
				Str("workflowId", hex.EncodeToString(wf.WorkflowId[:])).
				Msg("Failed to delete workflow")
			continue
		}
		switch txOut.Type {
		case client.Regular:
			h.log.Info().Msgf("Transaction confirmed: %s", txOut.Hash)
			h.log.Info().Msgf("Deleted workflow ID: %s", hex.EncodeToString(wf.WorkflowId[:]))

		case client.Raw:
			h.log.Info().Msg("")
			h.log.Info().Msg("MSIG workflow deletion transaction prepared!")
			h.log.Info().Msg("")
			h.log.Info().Msg("Next steps:")
			h.log.Info().Msg("")
			h.log.Info().Msg("   1. Submit the following transaction on the target chain:")
			h.log.Info().Msgf("      Chain:   %s", h.inputs.WorkflowRegistryContractChainName)
			h.log.Info().Msgf("      Contract Address: %s", txOut.RawTx.To)
			h.log.Info().Msg("")
			h.log.Info().Msg("   2. Use the following transaction data:")
			h.log.Info().Msg("")
			h.log.Info().Msgf("      %x", txOut.RawTx.Data)
			h.log.Info().Msg("")
		default:
			h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
		}

		// Workflow artifacts deletion will be handled by a background cleanup process.
	}
	h.log.Info().Msg("Workflows deleted successfully.")
	return nil
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
	promptWarning := fmt.Sprintf("Are you sure you want to delete the workflow '%s'?\n%s\n", expectedWorkflowName, text.FgRed.Sprint("This action cannot be undone."))
	h.log.Info().Msg(promptWarning)

	promptText := fmt.Sprintf("To confirm, type the workflow name: %s", expectedWorkflowName)
	var result string
	err := prompt.SimplePrompt(h.stdin, promptText, func(input string) error {
		result = input
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to get workflow name confirmation: %w", err)
	}

	return result == expectedWorkflowName, nil
}

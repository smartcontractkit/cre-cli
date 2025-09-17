package delete

import (
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	workflow_registry_v2_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
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

	WorkflowRegistryContractAddress       string `validate:"required"`
	WorkflowRegistryContractChainselector uint64 `validate:"required"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes all versions of a workflow from the Workflow Registry",
		Long:  "Deletes all workflow versions matching the given name and owner address.",
		Args:  cobra.NoArgs,
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
	deleteCmd.Flags().BoolP("skip-confirmation", "y", false, "Force delete workflow without confirmation")

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
		WorkflowName:                          h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner:                         h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		SkipConfirmation:                      v.GetBool("skip-confirmation"),
		WorkflowRegistryContractChainselector: h.environmentSet.WorkflowRegistryChainSelector,
		WorkflowRegistryContractAddress:       h.environmentSet.WorkflowRegistryAddress,
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

	if h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerType == constants.WorkflowOwnerTypeMSIG {
		for _, wf := range allWorkflows {
			txData, err := packDeleteTxData(wf.WorkflowId)
			if err != nil {
				return fmt.Errorf("failed to pack delete tx: %w", err)
			}
			if err := h.logMSIGNextSteps(txData); err != nil {
				return fmt.Errorf("failed to log MSIG steps: %w", err)
			}
		}
		return nil
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
		if err := wrc.DeleteWorkflow(wf.WorkflowId); err != nil {
			h.log.Error().
				Err(err).
				Str("workflowId", hex.EncodeToString(wf.WorkflowId[:])).
				Msg("Failed to delete workflow")
			continue
		}
		h.log.Info().Msgf("Deleted workflow ID: %s", hex.EncodeToString(wf.WorkflowId[:]))
		// Workflow artifacts deletion will be handled by a background cleanup process.
	}
	h.log.Info().Msg("Workflows deleted successfully.")
	return nil
}

// TODO: DEVSVCS-2341 Refactor to use txOutput interface
func packDeleteTxData(workflowID [32]byte) (string, error) {
	contractABI, err := abi.JSON(strings.NewReader(workflow_registry_v2_wrapper.WorkflowRegistryMetaData.ABI))
	if err != nil {
		return "", fmt.Errorf("parse ABI: %w", err)
	}
	data, err := contractABI.Pack("deleteWorkflow", workflowID)
	if err != nil {
		return "", fmt.Errorf("pack data: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func (h *handler) logMSIGNextSteps(txData string) error {
	ChainName, err := settings.GetChainNameByChainSelector(h.inputs.WorkflowRegistryContractChainselector)
	if err != nil {
		h.log.Error().Err(err).Uint64("selector", h.inputs.WorkflowRegistryContractChainselector).Msg("failed to get chain name")
		return err
	}
	h.log.Info().Msg("")
	h.log.Info().Msg("MSIG workflow deletion transaction prepared!")
	h.log.Info().Msg("")
	h.log.Info().Msg("Next steps:")
	h.log.Info().Msg("")
	h.log.Info().Msg("   1. Submit the following transaction on the target chain:")
	h.log.Info().Msgf("      Chain:   %s", ChainName)
	h.log.Info().Msgf("      Contract Address: %s", h.inputs.WorkflowRegistryContractAddress)
	h.log.Info().Msg("")
	h.log.Info().Msg("   2. Use the following transaction data:")
	h.log.Info().Msg("")
	h.log.Info().Msgf("      %s", txData)
	h.log.Info().Msg("")
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

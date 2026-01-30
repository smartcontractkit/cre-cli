package delete

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/prompt"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/types"
	"github.com/smartcontractkit/cre-cli/internal/ui"
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
	wrc            *client.WorkflowRegistryV2Client
	runtimeContext *runtime.Context

	validated bool

	wg     sync.WaitGroup
	wrcErr error
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
		wg:             sync.WaitGroup{},
		wrcErr:         nil,
	}
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		wrc, err := h.clientFactory.NewWorkflowRegistryV2Client()
		if err != nil {
			h.wrcErr = fmt.Errorf("failed to create workflow registry client: %w", err)
			return
		}
		h.wrc = wrc
	}()

	return &h
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

	h.displayWorkflowDetails()

	h.wg.Wait()
	if h.wrcErr != nil {
		return h.wrcErr
	}

	allWorkflows, err := h.wrc.GetWorkflowListByOwnerAndName(workflowOwner, workflowName, big.NewInt(0), big.NewInt(100))
	if err != nil {
		return fmt.Errorf("failed to get workflow list: %w", err)
	}
	if len(allWorkflows) == 0 {
		ui.Warning(fmt.Sprintf("No workflows found for name: %s", workflowName))
		return nil
	}

	// Note: The way deploy is set up, there will only ever be one workflow in the command for now
	h.runtimeContext.Workflow.ID = hex.EncodeToString(allWorkflows[0].WorkflowId[:])

	ui.Bold(fmt.Sprintf("Found %d workflow(s) to delete for name: %s", len(allWorkflows), workflowName))
	for i, wf := range allWorkflows {
		status := map[uint8]string{0: "ACTIVE", 1: "PAUSED"}[wf.Status]
		ui.Print(fmt.Sprintf("   %d. Workflow", i+1))
		ui.Dim(fmt.Sprintf("      ID:              %s", hex.EncodeToString(wf.WorkflowId[:])))
		ui.Dim(fmt.Sprintf("      Owner:           %s", wf.Owner.Hex()))
		ui.Dim(fmt.Sprintf("      DON Family:      %s", wf.DonFamily))
		ui.Dim(fmt.Sprintf("      Tag:             %s", wf.Tag))
		ui.Dim(fmt.Sprintf("      Binary URL:      %s", wf.BinaryUrl))
		ui.Dim(fmt.Sprintf("      Workflow Status: %s", status))
		ui.Line()
	}

	shouldDeleteWorkflow, err := h.shouldDeleteWorkflow(h.inputs.SkipConfirmation, workflowName)
	if err != nil {
		return err
	}
	if !shouldDeleteWorkflow {
		ui.Warning("Workflow deletion canceled")
		return nil
	}

	ui.Dim(fmt.Sprintf("Deleting %d workflow(s)...", len(allWorkflows)))
	var errs []error
	for _, wf := range allWorkflows {
		txOut, err := h.wrc.DeleteWorkflow(wf.WorkflowId)
		if err != nil {
			h.log.Error().
				Err(err).
				Str("workflowId", hex.EncodeToString(wf.WorkflowId[:])).
				Msg("Failed to delete workflow")
			errs = append(errs, err)
			continue
		}
		switch txOut.Type {
		case client.Regular:
			ui.Success("Transaction confirmed")
			ui.URL(fmt.Sprintf("%s/tx/%s", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash))
			ui.Success(fmt.Sprintf("Deleted workflow ID: %s", hex.EncodeToString(wf.WorkflowId[:])))

		case client.Raw:
			ui.Line()
			ui.Success("MSIG workflow deletion transaction prepared!")
			ui.Line()
			ui.Bold("Next steps:")
			ui.Line()
			ui.Print("   1. Submit the following transaction on the target chain:")
			ui.Dim(fmt.Sprintf("      Chain:            %s", h.inputs.WorkflowRegistryContractChainName))
			ui.Dim(fmt.Sprintf("      Contract Address: %s", txOut.RawTx.To))
			ui.Line()
			ui.Print("   2. Use the following transaction data:")
			ui.Line()
			ui.Code(fmt.Sprintf("      %x", txOut.RawTx.Data))
			ui.Line()

		case client.Changeset:
			chainSelector, err := settings.GetChainSelectorByChainName(h.environmentSet.WorkflowRegistryChainName)
			if err != nil {
				return fmt.Errorf("failed to get chain selector for chain %q: %w", h.environmentSet.WorkflowRegistryChainName, err)
			}
			mcmsConfig, err := settings.GetMCMSConfig(h.settings, chainSelector)
			if err != nil {
				ui.Warning("MCMS config not found or is incorrect, skipping MCMS config in changeset")
			}
			cldSettings := h.settings.CLDSettings
			changesets := []types.Changeset{
				{
					DeleteWorkflow: &types.DeleteWorkflow{
						Payload: types.UserWorkflowDeleteInput{
							WorkflowID: h.runtimeContext.Workflow.ID,

							ChainSelector:             chainSelector,
							MCMSConfig:                mcmsConfig,
							WorkflowRegistryQualifier: cldSettings.WorkflowRegistryQualifier,
						},
					},
				},
			}
			csFile := types.NewChangesetFile(cldSettings.Environment, cldSettings.Domain, cldSettings.MergeProposals, changesets)

			var fileName string
			if cldSettings.ChangesetFile != "" {
				fileName = cldSettings.ChangesetFile
			} else {
				fileName = fmt.Sprintf("DeleteWorkflow_%s_%s.yaml", workflowName, time.Now().Format("20060102_150405"))
			}

			return cmdCommon.WriteChangesetFile(fileName, csFile, h.settings)

		default:
			h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
		}

		// Workflow artifacts deletion will be handled by a background cleanup process.
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to delete some workflows: %w", errors.Join(errs...))
	}
	ui.Success("Workflows deleted successfully")
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
	fmt.Println(promptWarning)

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

func (h *handler) displayWorkflowDetails() {
	ui.Line()
	ui.Title(fmt.Sprintf("Deleting Workflow: %s", h.inputs.WorkflowName))
	ui.Dim(fmt.Sprintf("Target:        %s", h.settings.User.TargetName))
	ui.Dim(fmt.Sprintf("Owner Address: %s", h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress))
	ui.Line()
}

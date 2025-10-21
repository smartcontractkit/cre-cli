package delete

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"

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
		Use:   "delete <workflow-folder-path>",
		Short: "Deletes all versions of a workflow from the Workflow Registry",
		Long:  "Deletes all workflow versions matching the given name and owner address.",
		Args:  cobra.ExactArgs(1),
		Example: `
		cre workflow delete ./my-workflow
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
	wrc            *client.WorkflowRegistryV2Client

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
		fmt.Printf("No workflows found for name: %s\n", workflowName)
		return nil
	}

	fmt.Printf("Found %d workflow(s) to delete for name: %s\n", len(allWorkflows), workflowName)
	for i, wf := range allWorkflows {
		status := map[uint8]string{0: "ACTIVE", 1: "PAUSED"}[wf.Status]
		fmt.Printf("   %d. Workflow\n", i+1)
		fmt.Printf("      ID:              %s\n", hex.EncodeToString(wf.WorkflowId[:]))
		fmt.Printf("      Owner:           %s\n", wf.Owner.Hex())
		fmt.Printf("      DON Family:      %s\n", wf.DonFamily)
		fmt.Printf("      Tag:             %s\n", wf.Tag)
		fmt.Printf("      Binary URL:      %s\n", wf.BinaryUrl)
		fmt.Printf("      Workflow Status: %s\n", status)
		fmt.Println("")
	}

	shouldDeleteWorkflow, err := h.shouldDeleteWorkflow(h.inputs.SkipConfirmation, workflowName)
	if err != nil {
		return err
	}
	if !shouldDeleteWorkflow {
		fmt.Println("Workflow deletion canceled")
		return nil
	}

	fmt.Printf("Deleting %d workflow(s)...\n", len(allWorkflows))
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
			fmt.Println("Transaction confirmed")
			fmt.Printf("View on explorer: \033]8;;%s/tx/%s\033\\%s/tx/%s\033]8;;\033\\\n", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash, h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash)
			fmt.Printf("[OK] Deleted workflow ID: %s\n", hex.EncodeToString(wf.WorkflowId[:]))

		case client.Raw:
			fmt.Println("")
			fmt.Println("MSIG workflow deletion transaction prepared!")
			fmt.Println("")
			fmt.Println("Next steps:")
			fmt.Println("")
			fmt.Println("   1. Submit the following transaction on the target chain:")
			fmt.Printf("      Chain:   %s\n", h.inputs.WorkflowRegistryContractChainName)
			fmt.Printf("      Contract Address: %s\n", txOut.RawTx.To)
			fmt.Println("")
			fmt.Println("   2. Use the following transaction data:")
			fmt.Println("")
			fmt.Printf("      %x\n", txOut.RawTx.Data)
			fmt.Println("")
		default:
			h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
		}

		// Workflow artifacts deletion will be handled by a background cleanup process.
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to delete some workflows: %v", errors.Join(errs...))
	}
	fmt.Println("Workflows deleted successfully.")
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
	fmt.Printf("\nDeleting Workflow : \t %s\n", h.inputs.WorkflowName)
	fmt.Printf("Target : \t\t %s\n", h.settings.User.TargetName)
	fmt.Printf("Owner Address : \t %s\n\n", h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress)
}

package pause

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	workflow_registry_v2_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

const (
	WorkflowStatusActive = uint8(0)
)

type Inputs struct {
	WorkflowName                      string `validate:"workflow_name"`
	WorkflowOwner                     string `validate:"workflow_owner"`
	WorkflowRegistryContractAddress   string `validate:"required"`
	WorkflowRegistryContractChainName string `validate:"required"`
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

	settings.AddRawTxFlag(pauseCmd)
	settings.AddSkipConfirmation(pauseCmd)
	return pauseCmd
}

type handler struct {
	log            *zerolog.Logger
	clientFactory  client.Factory
	settings       *settings.Settings
	environmentSet *environments.EnvironmentSet
	inputs         Inputs
	wrc            *client.WorkflowRegistryV2Client

	validated bool

	wg     sync.WaitGroup
	wrcErr error
}

func newHandler(ctx *runtime.Context) *handler {
	h := handler{
		log:            ctx.Logger,
		clientFactory:  ctx.ClientFactory,
		settings:       ctx.Settings,
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
	if !h.validated {
		return fmt.Errorf("handler inputs not validated")
	}

	workflowName := h.inputs.WorkflowName
	workflowOwner := common.HexToAddress(h.inputs.WorkflowOwner)

	h.displayWorkflowDetails()

	h.wg.Wait()
	if h.wrcErr != nil {
		return h.wrcErr
	}

	fmt.Printf("Fetching workflows to pause... Name=%s, Owner=%s\n", workflowName, workflowOwner.Hex())

	workflows, err := fetchAllWorkflows(h.wrc, workflowOwner, workflowName)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}
	if len(workflows) == 0 {
		return fmt.Errorf("no workflows found for name %q and owner %q", workflowName, workflowOwner.Hex())
	}

	// Validate precondition: only pause workflows that are currently active
	var activeWorkflowIDs [][32]byte
	for _, workflow := range workflows {
		if workflow.Status == WorkflowStatusActive {
			activeWorkflowIDs = append(activeWorkflowIDs, workflow.WorkflowId)
		}
	}

	if len(activeWorkflowIDs) == 0 {
		return fmt.Errorf("workflow is already paused, cancelling transaction")
	}

	fmt.Printf("Processing batch pause... count=%d\n", len(activeWorkflowIDs))

	txOut, err := h.wrc.BatchPauseWorkflows(activeWorkflowIDs)
	if err != nil {
		return fmt.Errorf("failed to batch pause workflows: %w", err)
	}

	switch txOut.Type {
	case client.Regular:
		fmt.Println("Transaction confirmed")
		fmt.Printf("View on explorer: \033]8;;%s/tx/%s\033\\%s/tx/%s\033]8;;\033\\\n", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash, h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash)
		fmt.Println("[OK] Workflows paused successfully")
		fmt.Println("\nDetails:")
		fmt.Printf("   Contract address:\t%s\n", h.environmentSet.WorkflowRegistryAddress)
		fmt.Printf("   Transaction hash:\t%s\n", txOut.Hash)
		fmt.Printf("   Workflow Name:\t%s\n", workflowName)
		for _, w := range activeWorkflowIDs {
			fmt.Printf("   Workflow ID:\t%s\n", hex.EncodeToString(w[:]))
		}

	case client.Raw:
		fmt.Println("")
		fmt.Println("MSIG workflow pause transaction prepared!")
		fmt.Printf("To Pause %s\n", workflowName)
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
	return nil
}

func fetchAllWorkflows(
	wrc interface {
		GetWorkflowListByOwnerAndName(owner common.Address, workflowName string, start, limit *big.Int) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error)
	},
	owner common.Address,
	name string,
) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error) {
	const pageSize = int64(200)
	var (
		start     = big.NewInt(0)
		limit     = big.NewInt(pageSize)
		workflows = make([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, 0, pageSize)
	)

	for {
		list, err := wrc.GetWorkflowListByOwnerAndName(owner, name, start, limit)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			break
		}

		workflows = append(workflows, list...)

		start = big.NewInt(start.Int64() + int64(len(list)))
		if int64(len(list)) < pageSize {
			break
		}
	}

	return workflows, nil
}

func (h *handler) displayWorkflowDetails() {
	fmt.Printf("\nPausing Workflow : \t %s\n", h.inputs.WorkflowName)
	fmt.Printf("Target : \t\t %s\n", h.settings.User.TargetName)
	fmt.Printf("Owner Address : \t %s\n\n", h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress)
}

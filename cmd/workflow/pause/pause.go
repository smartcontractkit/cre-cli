package pause

import (
	"encoding/hex"
	"fmt"
	"math/big"

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

type Inputs struct {
	WorkflowName                      string `validate:"workflow_name"`
	WorkflowOwner                     string `validate:"workflow_owner"`
	WorkflowRegistryContractAddress   string `validate:"required"`
	WorkflowRegistryContractChainName string `validate:"required"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var pauseCmd = &cobra.Command{
		Use:   "pause",
		Short: "Pauses workflow on the Workflow Registry contract",
		Long:  `Changes workflow status to paused on the Workflow Registry contract`,
		Args:  cobra.NoArgs,
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
	validated      bool
	environmentSet *environments.EnvironmentSet
	inputs         Inputs
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:            ctx.Logger,
		clientFactory:  ctx.ClientFactory,
		settings:       ctx.Settings,
		environmentSet: ctx.EnvironmentSet,
		validated:      false,
	}
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

	wrc, err := h.clientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("failed to create workflow registry client: %w", err)
	}

	fmt.Printf("Fetching workflows to pause... Name=%s, Owner=%s\n", workflowName, workflowOwner.Hex())

	workflowIDs, err := fetchAllWorkflowIDs(wrc, workflowOwner, workflowName)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}
	if len(workflowIDs) == 0 {
		return fmt.Errorf("no workflows found for name %q and owner %q", workflowName, workflowOwner.Hex())
	}

	fmt.Printf("Processing batch pause... count=%d\n", len(workflowIDs))

	txOut, err := wrc.BatchPauseWorkflows(workflowIDs)
	if err != nil {
		return fmt.Errorf("failed to batch pause workflows: %w", err)
	}

	switch txOut.Type {
	case client.Regular:
		fmt.Printf("Transaction confirmed: %s\n", txOut.Hash)
		fmt.Printf("View on explorer: \033]8;;%s/tx/%s\033\\%s/tx/%s\033]8;;\033\\\n", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash, h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash)
		for _, w := range workflowIDs {
			fmt.Printf("Paused workflow IDs: %s\n", hex.EncodeToString(w[:]))
		}
		fmt.Println("Workflows paused successfully")

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

func fetchAllWorkflowIDs(
	wrc interface {
		GetWorkflowListByOwnerAndName(owner common.Address, workflowName string, start, limit *big.Int) ([]workflow_registry_v2_wrapper.WorkflowRegistryWorkflowMetadataView, error)
	},
	owner common.Address,
	name string,
) ([][32]byte, error) {
	const pageSize = int64(100)
	var (
		start = big.NewInt(0)
		limit = big.NewInt(pageSize)
		ids   = make([][32]byte, 0, pageSize)
	)

	for {
		list, err := wrc.GetWorkflowListByOwnerAndName(owner, name, start, limit)
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			break
		}

		for _, m := range list {
			ids = append(ids, m.WorkflowId)
		}

		start = big.NewInt(start.Int64() + int64(len(list)))
		if int64(len(list)) < pageSize {
			break
		}
	}

	return ids, nil
}

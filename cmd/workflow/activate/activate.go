package activate

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	WorkflowName                      string `validate:"workflow_name"`
	WorkflowOwner                     string `validate:"workflow_owner"`
	DonFamily                         string `validate:"required"`
	WorkflowRegistryContractAddress   string `validate:"required"`
	WorkflowRegistryContractChainName string `validate:"required"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	activateCmd := &cobra.Command{
		Use:   "activate",
		Short: "Activates workflow on the Workflow Registry contract",
		Long:  `Changes workflow status to active on the Workflow Registry contract`,
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

	settings.AddRawTxFlag(activateCmd)
	settings.AddNonInteractiveFlag(activateCmd)

	return activateCmd
}

type handler struct {
	log             *zerolog.Logger
	clientFactory   client.Factory
	settings        *settings.Settings
	environmentsSet *environments.EnvironmentSet
	validated       bool
	inputs          Inputs
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:             ctx.Logger,
		clientFactory:   ctx.ClientFactory,
		settings:        ctx.Settings,
		environmentsSet: ctx.EnvironmentSet,
		validated:       false,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	return Inputs{
		WorkflowName:                      h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner:                     h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		DonFamily:                         h.settings.Workflow.DevPlatformSettings.DonFamily,
		WorkflowRegistryContractAddress:   h.environmentsSet.WorkflowRegistryAddress,
		WorkflowRegistryContractChainName: h.environmentsSet.WorkflowRegistryChainName,
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
	workflowOwner := h.inputs.WorkflowOwner

	wrc, err := h.clientFactory.NewWorkflowRegistryV2Client()
	if err != nil {
		return fmt.Errorf("failed to create WorkflowRegistryClient: %w", err)
	}

	ownerAddr := common.HexToAddress(workflowOwner)

	const pageLimit = 200
	workflows, err := wrc.GetWorkflowListByOwnerAndName(ownerAddr, workflowName, big.NewInt(0), big.NewInt(pageLimit))
	if err != nil {
		return fmt.Errorf("failed to get workflow list: %w", err)
	}
	if len(workflows) == 0 {
		return fmt.Errorf("no workflows found for owner=%s name=%s", workflowOwner, workflowName)
	}

	// Sort by CreatedAt descending
	sort.Slice(workflows, func(i, j int) bool {
		return workflows[i].CreatedAt > workflows[j].CreatedAt
	})

	latest := workflows[0]

	h.log.Info().
		Str("Name", workflowName).
		Str("Owner", workflowOwner).
		Str("WorkflowID", hex.EncodeToString(latest.WorkflowId[:])).
		Msg("Activating workflow")

	txOut, err := wrc.ActivateWorkflow(latest.WorkflowId, h.inputs.DonFamily)
	if err != nil {
		return fmt.Errorf("failed to activate workflow: %w", err)
	}

	switch txOut.Type {
	case client.Regular:
		h.log.Info().Msgf("Transaction confirmed: %s", txOut.Hash)
		h.log.Info().Msgf("Activated workflow ID: %s", hex.EncodeToString(latest.WorkflowId[:]))
		h.log.Info().Msg("Workflow activated successfully")

	case client.Raw:
		h.log.Info().Msg("")
		h.log.Info().Msg("MSIG workflow activation transaction prepared!")
		h.log.Info().Msgf("To Activate %s with workflowID: %s", workflowName, hex.EncodeToString(latest.WorkflowId[:]))
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
	return nil
}

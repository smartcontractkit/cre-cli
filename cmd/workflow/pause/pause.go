package pause

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	workflow_registry_v2_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	WorkflowName                          string `validate:"workflow_name"`
	WorkflowOwner                         string `validate:"workflow_owner"`
	WorkflowRegistryContractAddress       string `validate:"required"`
	WorkflowRegistryContractChainselector uint64 `validate:"required"`
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
	return pauseCmd
}

type handler struct {
	log             *zerolog.Logger
	clientFactory   client.Factory
	settings        *settings.Settings
	validated       bool
	environmentsSet *environments.EnvironmentSet
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
		WorkflowName:                          h.settings.Workflow.UserWorkflowSettings.WorkflowName,
		WorkflowOwner:                         h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerAddress,
		WorkflowRegistryContractChainselector: h.environmentsSet.WorkflowRegistryChainSelector,
		WorkflowRegistryContractAddress:       h.environmentsSet.WorkflowRegistryAddress,
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

	h.log.Info().
		Str("Name", workflowName).
		Str("Owner", workflowOwner.Hex()).
		Msg("Fetching workflows to pause...")

	workflowIDs, err := fetchAllWorkflowIDs(wrc, workflowOwner, workflowName)
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}
	if len(workflowIDs) == 0 {
		return fmt.Errorf("no workflows found for name %q and owner %q", workflowName, workflowOwner.Hex())
	}

	if h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerType == constants.WorkflowOwnerTypeMSIG {
		txData, err := packBatchPauseTxData(workflowIDs)
		if err != nil {
			return fmt.Errorf("failed to pack batch pause tx: %w", err)
		}
		if err := h.logMSIGNextSteps(h.inputs, txData); err != nil {
			return fmt.Errorf("failed to log MSIG steps: %w", err)
		}
		return nil
	}

	h.log.Info().
		Int("count", len(workflowIDs)).
		Msg("Submitting batch pause...")

	if err := wrc.BatchPauseWorkflows(workflowIDs); err != nil {
		return fmt.Errorf("failed to batch pause workflows: %w", err)
	}

	h.log.Info().Msg("Workflows paused successfully")
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

// TODO: DEVSVCS-2341 Refactor to use txOutput interface
func packBatchPauseTxData(ids [][32]byte) (string, error) {
	contractABI, err := abi.JSON(strings.NewReader(workflow_registry_v2_wrapper.WorkflowRegistryMetaData.ABI))
	if err != nil {
		return "", fmt.Errorf("parse ABI: %w", err)
	}
	data, err := contractABI.Pack("batchPauseWorkflows", ids)
	if err != nil {
		return "", fmt.Errorf("pack data: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func (h *handler) logMSIGNextSteps(inputs Inputs, txData string) error {
	chainName, err := settings.GetChainNameByChainSelector(inputs.WorkflowRegistryContractChainselector)
	if err != nil {
		h.log.Error().Err(err).Uint64("selector", inputs.WorkflowRegistryContractChainselector).Msg("failed to get chain name")
		return err
	}

	h.log.Info().Msg("")
	h.log.Info().Msg("MSIG workflow pause transaction prepared!")
	h.log.Info().Msg("")
	h.log.Info().Msg("Next steps:")
	h.log.Info().Msg("")
	h.log.Info().Msg("   1. Submit the following transaction on the target chain:")
	h.log.Info().Msgf("      Chain:   %s", chainName)
	h.log.Info().Msgf("      Contract Address: %s", inputs.WorkflowRegistryContractAddress)
	h.log.Info().Msg("")
	h.log.Info().Msg("   2. Use the following transaction data:")
	h.log.Info().Msg("")
	h.log.Info().Msgf("      %s", txData)
	h.log.Info().Msg("")
	return nil
}

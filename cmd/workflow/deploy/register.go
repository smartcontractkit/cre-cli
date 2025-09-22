package deploy

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	workflow_registry_v2_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
)

func (h *handler) upsert() error {
	if !h.validated {
		return fmt.Errorf("handler inputs not validated")
	}

	params, err := h.prepareUpsertParams()
	if err != nil {
		return err
	}
	return h.submitWorkflow(params)
}

func (h *handler) submitWorkflow(params client.RegisterWorkflowV2Parameters) error {
	if h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerType == constants.WorkflowOwnerTypeMSIG {
		return h.handleMSIGFlow(params)
	}
	return h.handleEOAFlow(params)
}

func (h *handler) prepareUpsertParams() (client.RegisterWorkflowV2Parameters, error) {
	workflowName := h.inputs.WorkflowName
	workflowTag := h.inputs.WorkflowTag
	binaryURL := h.inputs.BinaryURL
	configURL := h.inputs.ResolveConfigURL("")
	workflowID := h.workflowArtifact.WorkflowID

	h.log.Info().Str("workflowID", workflowID).Msg("Preparing transaction...")
	return client.RegisterWorkflowV2Parameters{
		WorkflowName: workflowName,
		Tag:          workflowTag,
		WorkflowID:   [32]byte(common.Hex2Bytes(workflowID)),
		Status:       getWorkflowInitialStatus(h.inputs.AutoStart),
		DonFamily:    h.inputs.DonFamily,
		BinaryURL:    binaryURL,
		ConfigURL:    configURL,
		Attributes:   []byte{}, // optional
		KeepAlive:    h.inputs.KeepAlive,
	}, nil
}

func (h *handler) handleMSIGFlow(params client.RegisterWorkflowV2Parameters) error {
	h.log.Debug().Interface("Workflow parameters", params).Msg("Preparing workflow...")
	txData, err := packUpsertTxData(params)
	if err != nil {
		return fmt.Errorf("failed to pack transaction data: %w", err)
	}
	if err := h.logMSIGNextSteps(txData); err != nil {
		return fmt.Errorf("failed to prepare transaction data: %w", err)
	}
	return nil
}

func (h *handler) handleEOAFlow(params client.RegisterWorkflowV2Parameters) error {
	h.log.Debug().Interface("Workflow parameters", params).Msg("Registering workflow...")
	if err := h.wrc.UpsertWorkflow(params); err != nil {
		return fmt.Errorf("failed to register workflow: %w", err)
	}
	return nil
}

// TODO: DEVSVCS-2341 Refactor to use txOutput interface
func packUpsertTxData(params client.RegisterWorkflowV2Parameters) (string, error) {
	contractABI, err := abi.JSON(strings.NewReader(workflow_registry_v2_wrapper.WorkflowRegistryMetaData.ABI))
	if err != nil {
		return "", fmt.Errorf("parse ABI: %w", err)
	}

	data, err := contractABI.Pack(
		"upsertWorkflow",
		params.WorkflowName,
		params.Tag,
		params.WorkflowID,
		params.Status,
		params.DonFamily,
		params.BinaryURL,
		params.ConfigURL,
		params.Attributes,
		params.KeepAlive,
	)
	if err != nil {
		return "", fmt.Errorf("pack data: %w", err)
	}

	return hex.EncodeToString(data), nil
}

func (h *handler) logMSIGNextSteps(txData string) error {
	h.log.Info().Msg("")
	h.log.Info().Msg("MSIG workflow deployment transaction prepared!")
	h.log.Info().Msg("")
	h.log.Info().Msg("Next steps:")
	h.log.Info().Msg("")
	h.log.Info().Msg("   1. Submit the following transaction on the target chain:")
	h.log.Info().Msgf("      Chain:   %s", h.inputs.WorkflowRegistryContractChainName)
	h.log.Info().Msgf("      Contract Address: %s", h.inputs.WorkflowRegistryContractAddress)
	h.log.Info().Msg("")
	h.log.Info().Msg("   2. Use the following transaction data:")
	h.log.Info().Msg("")
	h.log.Info().Msgf("      %s", txData)
	h.log.Info().Msg("")

	return nil
}

func getWorkflowInitialStatus(autoStart bool) uint8 {
	if autoStart {
		return 0 // active
	}
	return 1 // paused
}

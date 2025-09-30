package deploy

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/cre-cli/cmd/client"
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
	return h.handleUpsert(params)
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

func (h *handler) handleUpsert(params client.RegisterWorkflowV2Parameters) error {
	workflowName := h.inputs.WorkflowName
	workflowTag := h.inputs.WorkflowTag
	h.log.Debug().Interface("Workflow parameters", params).Msg("Registering workflow...")
	txOut, err := h.wrc.UpsertWorkflow(params)
	if err != nil {
		return fmt.Errorf("failed to register workflow: %w", err)
	}
	switch txOut.Type {
	case client.Regular:
		h.log.Info().Msgf("Transaction confirmed: %s", txOut.Hash)
		h.log.Info().Msgf("View on explorer: %s/tx/%s", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash)
		h.log.Info().Msgf("Deployed %s:%s with workflow ID: %s", workflowName, workflowTag, hex.EncodeToString(params.WorkflowID[:]))
		h.log.Info().Msg("Workflow deployed successfully")

	case client.Raw:
		h.log.Info().Msg("")
		h.log.Info().Msg("MSIG workflow deployment transaction prepared!")
		h.log.Info().Msgf("To Deploy %s:%s with workflow ID: %s", workflowName, workflowTag, hex.EncodeToString(params.WorkflowID[:]))
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

func getWorkflowInitialStatus(autoStart bool) uint8 {
	if autoStart {
		return 0 // active
	}
	return 1 // paused
}

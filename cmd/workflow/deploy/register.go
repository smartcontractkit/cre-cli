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

	fmt.Printf("Preparing transaction for workflowID: %s\n", workflowID)
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
		fmt.Printf("Transaction confirmed: %s\n", txOut.Hash)
		fmt.Printf("View on explorer: \033]8;;%s/tx/%s\033\\%s/tx/%s\033]8;;\033\\\n", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash, h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash)
		fmt.Printf("Deployed %s:%s with workflow ID: %s\n", workflowName, workflowTag, hex.EncodeToString(params.WorkflowID[:]))
		fmt.Println("Workflow deployed successfully")

	case client.Raw:
		fmt.Println("")
		fmt.Println("MSIG workflow deployment transaction prepared!")
		fmt.Printf("To Deploy %s:%s with workflow ID: %s\n", workflowName, workflowTag, hex.EncodeToString(params.WorkflowID[:]))
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

func getWorkflowInitialStatus(autoStart bool) uint8 {
	if autoStart {
		return 0 // active
	}
	return 1 // paused
}

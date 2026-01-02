package deploy

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/chainlink/deployment/cre/workflow_registry/v2/changeset"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/types"
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
		Status:       0, // active
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
		fmt.Println("Transaction confirmed")
		fmt.Printf("View on explorer: \033]8;;%s/tx/%s\033\\%s/tx/%s\033]8;;\033\\\n", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash, h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash)
		fmt.Println("\n[OK] Workflow deployed successfully")
		fmt.Println("\nDetails:")
		fmt.Printf("   Contract address:\t%s\n", h.environmentSet.WorkflowRegistryAddress)
		fmt.Printf("   Transaction hash:\t%s\n", txOut.Hash)
		fmt.Printf("   Workflow Name:\t%s\n", workflowName)
		fmt.Printf("   Workflow ID:\t%s\n", h.workflowArtifact.WorkflowID)
		fmt.Printf("   Binary URL:\t%s\n", h.inputs.BinaryURL)
		if h.inputs.ConfigURL != nil && *h.inputs.ConfigURL != "" {
			fmt.Printf("   Config URL:\t%s\n", *h.inputs.ConfigURL)
		}

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

	case client.Changeset:
		chainSelector, err := settings.GetChainSelectorByChainName(h.environmentSet.WorkflowRegistryChainName)
		if err != nil {
			return fmt.Errorf("failed to get chain selector for chain %q: %w", h.environmentSet.WorkflowRegistryChainName, err)
		}
		mcmsConfig, err := types.MCMSConfig(h.settings, chainSelector)
		if err != nil {
			return fmt.Errorf("failed to get MCMS config: %w", err)
		}
		csFile := types.ChangesetFile{
			Environment: h.settings.Workflow.CLDSettings.Environment,
			Domain:      h.settings.Workflow.CLDSettings.Domain,
			Changesets: []types.Changeset{
				{
					UpsertWorkflow: &types.UpsertWorkflow{
						Payload: changeset.UserWorkflowUpsertInput{
							WorkflowID:     h.runtimeContext.Workflow.ID,
							WorkflowName:   params.WorkflowName,
							WorkflowTag:    params.Tag,
							WorkflowStatus: params.Status,
							DonFamily:      params.DonFamily,
							BinaryURL:      params.BinaryURL,
							ConfigURL:      params.ConfigURL,
							Attributes:     common.Bytes2Hex(params.Attributes),
							KeepAlive:      params.KeepAlive,

							ChainSelector:             chainSelector,
							MCMSConfig:                mcmsConfig,
							WorkflowRegistryQualifier: h.settings.Workflow.CLDSettings.WorkflowRegistryQualifier,
						},
					},
				},
			},
		}

		fileName := fmt.Sprintf("UpsertWorkflow_%s_%s.yaml", workflowName, time.Now().Format("20060102_150405"))

		return types.WriteChangesetFile(fileName, &csFile, h.settings)

	default:
		h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
	}
	return nil
}

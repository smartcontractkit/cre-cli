package deploy

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/types"
	"github.com/smartcontractkit/cre-cli/internal/ui"
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

	// Use the existing workflow's status if updating, otherwise default to active (0).
	status := uint8(0)
	if h.existingWorkflowStatus != nil {
		status = *h.existingWorkflowStatus
	}

	attrs, err := h.buildAttributes()
	if err != nil {
		return client.RegisterWorkflowV2Parameters{}, err
	}

	ui.Dim(fmt.Sprintf("Preparing transaction for workflowID: %s", workflowID))
	return client.RegisterWorkflowV2Parameters{
		WorkflowName: workflowName,
		Tag:          workflowTag,
		WorkflowID:   [32]byte(common.Hex2Bytes(workflowID)),
		Status:       status,
		DonFamily:    h.inputs.DonFamily,
		BinaryURL:    binaryURL,
		ConfigURL:    configURL,
		Attributes:   attrs,
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
		ui.Success("Transaction confirmed")
		ui.URL(fmt.Sprintf("%s/tx/%s", h.environmentSet.WorkflowRegistryChainExplorerURL, txOut.Hash))
		ui.Line()
		ui.Success("Workflow deployed successfully")
		ui.Line()
		ui.Bold("Details:")
		ui.Dim(fmt.Sprintf("   Contract address: %s", h.environmentSet.WorkflowRegistryAddress))
		ui.Dim(fmt.Sprintf("   Transaction hash: %s", txOut.Hash))
		ui.Dim(fmt.Sprintf("   Workflow Name:    %s", workflowName))
		ui.Dim(fmt.Sprintf("   Workflow ID:      %s", h.workflowArtifact.WorkflowID))
		ui.Dim(fmt.Sprintf("   Binary URL:       %s", h.inputs.BinaryURL))
		if h.inputs.ConfigURL != nil && *h.inputs.ConfigURL != "" {
			ui.Dim(fmt.Sprintf("   Config URL:       %s", *h.inputs.ConfigURL))
		}

	case client.Raw:
		ui.Line()
		ui.Success("MSIG workflow deployment transaction prepared!")
		ui.Dim(fmt.Sprintf("To Deploy %s:%s with workflow ID: %s", workflowName, workflowTag, hex.EncodeToString(params.WorkflowID[:])))
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
				UpsertWorkflow: &types.UpsertWorkflow{
					Payload: types.UserWorkflowUpsertInput{
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
			fileName = fmt.Sprintf("UpsertWorkflow_%s_%s.yaml", workflowName, time.Now().Format("20060102_150405"))
		}

		return cmdCommon.WriteChangesetFile(fileName, csFile, h.settings)

	default:
		h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
	}
	return nil
}

func (h *handler) buildAttributes() ([]byte, error) {
	if !h.inputs.Confidential {
		return []byte{}, nil
	}

	secrets := make([]secretIdentifier, 0, len(h.inputs.Secrets))
	for _, s := range h.inputs.Secrets {
		key, ns, _ := strings.Cut(s, ":")
		secrets = append(secrets, secretIdentifier{
			Key:       key,
			Namespace: ns,
		})
	}

	attrs := workflowAttributes{
		Confidential:    true,
		VaultDonSecrets: secrets,
	}

	data, err := json.Marshal(attrs)
	if err != nil {
		return nil, fmt.Errorf("marshalling workflow attributes: %w", err)
	}
	return data, nil
}

type workflowAttributes struct {
	Confidential    bool               `json:"confidential"`
	VaultDonSecrets []secretIdentifier `json:"vault_don_secrets,omitempty"`
}

type secretIdentifier struct {
	Key       string `json:"key"`
	Namespace string `json:"namespace,omitempty"`
}

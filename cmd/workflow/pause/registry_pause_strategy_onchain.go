package pause

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"

	workflow_registry_v2_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/types"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

type onchainRegistryPauseStrategy struct {
	h       *handler
	wrc     *client.WorkflowRegistryV2Client
	onChain *settings.OnChainRegistry
	wg      sync.WaitGroup
	initErr error
}

func newOnchainRegistryPauseStrategy(h *handler) (*onchainRegistryPauseStrategy, error) {
	onChain, err := settings.AsOnChain(h.runtimeContext.ResolvedRegistry, "pause")
	if err != nil {
		return nil, err
	}

	a := &onchainRegistryPauseStrategy{h: h, onChain: onChain}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		wrc, err := h.clientFactory.NewWorkflowRegistryV2Client()
		if err != nil {
			a.initErr = fmt.Errorf("failed to create workflow registry client: %w", err)
			return
		}
		a.wrc = wrc
	}()
	return a, nil
}

func (a *onchainRegistryPauseStrategy) Pause() error {
	h := a.h

	a.wg.Wait()
	if a.initErr != nil {
		return a.initErr
	}

	workflowName := h.inputs.WorkflowName
	workflowOwner := common.HexToAddress(h.inputs.WorkflowOwner)

	ui.Dim(fmt.Sprintf("Fetching workflows to pause... Name=%s, Owner=%s", workflowName, workflowOwner.Hex()))

	workflows, err := fetchAllWorkflows(a.wrc, workflowOwner, workflowName)
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

	// Note: The way deploy is set up, there will only ever be one workflow in the command for now
	h.runtimeContext.Workflow.ID = hex.EncodeToString(activeWorkflowIDs[0][:])

	ui.Dim(fmt.Sprintf("Processing batch pause... count=%d", len(activeWorkflowIDs)))

	txOut, err := a.wrc.BatchPauseWorkflows(activeWorkflowIDs)
	if err != nil {
		return fmt.Errorf("failed to batch pause workflows: %w", err)
	}

	oc := a.onChain

	switch txOut.Type {
	case client.Regular:
		ui.Success("Transaction confirmed")
		ui.URL(fmt.Sprintf("%s/tx/%s", oc.ExplorerURL(), txOut.Hash))
		ui.Success("Workflows paused successfully")
		ui.Line()
		ui.Bold("Details:")
		ui.Dim(fmt.Sprintf("   Registry:         %s", h.runtimeContext.ResolvedRegistry.ID()))
		ui.Dim(fmt.Sprintf("   Contract address: %s", oc.Address()))
		ui.Dim(fmt.Sprintf("   Transaction hash: %s", txOut.Hash))
		ui.Dim(fmt.Sprintf("   Workflow Name:    %s", workflowName))
		for _, w := range activeWorkflowIDs {
			ui.Dim(fmt.Sprintf("   Workflow ID:      %s", hex.EncodeToString(w[:])))
		}

	case client.Raw:
		ui.Line()
		ui.Success("MSIG workflow pause transaction prepared!")
		ui.Dim(fmt.Sprintf("To Pause %s", workflowName))
		ui.Line()
		ui.Bold("Next steps:")
		ui.Line()
		ui.Print("   1. Submit the following transaction on the target chain:")
		ui.Dim(fmt.Sprintf("      Chain:            %s", oc.ChainName()))
		ui.Dim(fmt.Sprintf("      Contract Address: %s", txOut.RawTx.To))
		ui.Line()
		ui.Print("   2. Use the following transaction data:")
		ui.Line()
		ui.Code(fmt.Sprintf("      %x", txOut.RawTx.Data))
		ui.Line()

	case client.Changeset:
		chainSelector, err := settings.GetChainSelectorByChainName(oc.ChainName())
		if err != nil {
			return fmt.Errorf("failed to get chain selector for chain %q: %w", oc.ChainName(), err)
		}
		mcmsConfig, err := settings.GetMCMSConfig(h.settings, chainSelector)
		if err != nil {
			ui.Warning("MCMS config not found or is incorrect, skipping MCMS config in changeset")
		}
		cldSettings := h.settings.CLDSettings
		changesets := []types.Changeset{
			{
				BatchPauseWorkflow: &types.BatchPauseWorkflow{
					Payload: types.UserWorkflowBatchPauseInput{
						WorkflowIDs: h.runtimeContext.Workflow.ID, // Note: The way deploy is set up, there will only ever be one workflow in the command for now

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
			fileName = fmt.Sprintf("BatchPauseWorkflow_%s_%s.yaml", workflowName, time.Now().Format("20060102_150405"))
		}

		return cmdCommon.WriteChangesetFile(fileName, csFile, h.settings)

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

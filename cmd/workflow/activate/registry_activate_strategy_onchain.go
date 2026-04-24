package activate

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/types"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

type onchainRegistryActivateStrategy struct {
	h       *handler
	wrc     *client.WorkflowRegistryV2Client
	onChain *settings.OnChainRegistry
	wg      sync.WaitGroup
	initErr error
}

func newOnchainRegistryActivateStrategy(h *handler) (*onchainRegistryActivateStrategy, error) {
	onChain, err := settings.AsOnChain(h.runtimeContext.ResolvedRegistry, "activate")
	if err != nil {
		return nil, err
	}

	a := &onchainRegistryActivateStrategy{h: h, onChain: onChain}
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

func (a *onchainRegistryActivateStrategy) Activate() error {
	h := a.h

	a.wg.Wait()
	if a.initErr != nil {
		return a.initErr
	}

	workflowName := h.inputs.WorkflowName
	workflowOwner := h.inputs.WorkflowOwner

	ownerAddr := common.HexToAddress(workflowOwner)

	const pageLimit = 200
	workflows, err := a.wrc.GetWorkflowListByOwnerAndName(ownerAddr, workflowName, big.NewInt(0), big.NewInt(pageLimit))
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

	h.runtimeContext.Workflow.ID = hex.EncodeToString(latest.WorkflowId[:])

	// Validate precondition: workflow must be in paused state
	if latest.Status != WorkflowStatusPaused {
		return fmt.Errorf("workflow is already active, cancelling transaction")
	}

	if err := a.wrc.CheckUserDonLimit(ownerAddr, h.inputs.DonFamily, 1); err != nil {
		return err
	}

	ui.Dim(fmt.Sprintf("Activating workflow: Name=%s, Owner=%s, WorkflowID=%s", workflowName, workflowOwner, hex.EncodeToString(latest.WorkflowId[:])))

	txOut, err := a.wrc.ActivateWorkflow(latest.WorkflowId, h.inputs.DonFamily)
	if err != nil {
		return fmt.Errorf("failed to activate workflow: %w", err)
	}

	oc := a.onChain

	switch txOut.Type {
	case client.Regular:
		ui.Success(fmt.Sprintf("Transaction confirmed: %s", txOut.Hash))
		ui.URL(fmt.Sprintf("%s/tx/%s", oc.ExplorerURL(), txOut.Hash))
		ui.Line()
		ui.Success("Workflow activated successfully")
		ui.Line()
		ui.Bold("Details:")
		ui.Dim(fmt.Sprintf("   Registry:         %s", h.runtimeContext.ResolvedRegistry.ID()))
		ui.Dim(fmt.Sprintf("   Contract address: %s", oc.Address()))
		ui.Dim(fmt.Sprintf("   Transaction hash: %s", txOut.Hash))
		ui.Dim(fmt.Sprintf("   Workflow Name:    %s", workflowName))
		ui.Dim(fmt.Sprintf("   Workflow ID:      %s", hex.EncodeToString(latest.WorkflowId[:])))

	case client.Raw:
		ui.Line()
		ui.Success("MSIG workflow activation transaction prepared!")
		ui.Dim(fmt.Sprintf("To Activate %s with workflowID: %s", workflowName, hex.EncodeToString(latest.WorkflowId[:])))
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
				ActivateWorkflow: &types.ActivateWorkflow{
					Payload: types.UserWorkflowActivateInput{
						WorkflowID: h.runtimeContext.Workflow.ID,
						DonFamily:  h.inputs.DonFamily,

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
			fileName = fmt.Sprintf("ActivateWorkflow_%s_%s.yaml", workflowName, time.Now().Format("20060102_150405"))
		}

		return cmdCommon.WriteChangesetFile(fileName, csFile, h.settings)

	default:
		h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
	}
	return nil
}

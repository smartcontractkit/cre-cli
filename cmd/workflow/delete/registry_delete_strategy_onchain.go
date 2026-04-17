package delete

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	cmdCommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/types"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

type onchainRegistryDeleteStrategy struct {
	h       *handler
	wrc     *client.WorkflowRegistryV2Client
	onChain *settings.OnChainRegistry
	wg      sync.WaitGroup
	initErr error
}

func newOnchainRegistryDeleteStrategy(h *handler) (*onchainRegistryDeleteStrategy, error) {
	onChain, err := settings.AsOnChain(h.runtimeContext.ResolvedRegistry, "delete")
	if err != nil {
		return nil, err
	}

	a := &onchainRegistryDeleteStrategy{h: h, onChain: onChain}
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

func (a *onchainRegistryDeleteStrategy) Delete() error {
	h := a.h

	a.wg.Wait()
	if a.initErr != nil {
		return a.initErr
	}

	workflowName := h.inputs.WorkflowName
	workflowOwner := common.HexToAddress(h.inputs.WorkflowOwner)

	allWorkflows, err := a.wrc.GetWorkflowListByOwnerAndName(workflowOwner, workflowName, big.NewInt(0), big.NewInt(100))
	if err != nil {
		return fmt.Errorf("failed to get workflow list: %w", err)
	}
	if len(allWorkflows) == 0 {
		ui.Warning(fmt.Sprintf("No workflows found for name: %s", workflowName))
		return nil
	}

	// Note: The way deploy is set up, there will only ever be one workflow in the command for now
	h.runtimeContext.Workflow.ID = hex.EncodeToString(allWorkflows[0].WorkflowId[:])

	ui.Bold(fmt.Sprintf("Found %d workflow(s) to delete for name: %s", len(allWorkflows), workflowName))
	for i, wf := range allWorkflows {
		status := map[uint8]string{0: "ACTIVE", 1: "PAUSED"}[wf.Status]
		ui.Print(fmt.Sprintf("   %d. Workflow", i+1))
		ui.Dim(fmt.Sprintf("      ID:              %s", hex.EncodeToString(wf.WorkflowId[:])))
		ui.Dim(fmt.Sprintf("      Owner:           %s", wf.Owner.Hex()))
		ui.Dim(fmt.Sprintf("      DON Family:      %s", wf.DonFamily))
		ui.Dim(fmt.Sprintf("      Tag:             %s", wf.Tag))
		ui.Dim(fmt.Sprintf("      Binary URL:      %s", wf.BinaryUrl))
		ui.Dim(fmt.Sprintf("      Workflow Status: %s", status))
		ui.Line()
	}

	shouldDeleteWorkflow, err := h.shouldDeleteWorkflow(h.inputs.SkipConfirmation, workflowName)
	if err != nil {
		return err
	}
	if !shouldDeleteWorkflow {
		ui.Warning("Workflow deletion canceled")
		return nil
	}

	ui.Dim(fmt.Sprintf("Deleting %d workflow(s)...", len(allWorkflows)))
	var errs []error
	for _, wf := range allWorkflows {
		txOut, err := a.wrc.DeleteWorkflow(wf.WorkflowId)
		if err != nil {
			h.log.Error().
				Err(err).
				Str("workflowId", hex.EncodeToString(wf.WorkflowId[:])).
				Msg("Failed to delete workflow")
			errs = append(errs, err)
			continue
		}
		oc := a.onChain

		switch txOut.Type {
		case client.Regular:
			ui.Success("Transaction confirmed")
			ui.URL(fmt.Sprintf("%s/tx/%s", oc.ExplorerURL(), txOut.Hash))
			ui.Success(fmt.Sprintf("Deleted workflow ID: %s", hex.EncodeToString(wf.WorkflowId[:])))

		case client.Raw:
			ui.Line()
			ui.Success("MSIG workflow deletion transaction prepared!")
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
					DeleteWorkflow: &types.DeleteWorkflow{
						Payload: types.UserWorkflowDeleteInput{
							WorkflowID: h.runtimeContext.Workflow.ID,

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
				fileName = fmt.Sprintf("DeleteWorkflow_%s_%s.yaml", workflowName, time.Now().Format("20060102_150405"))
			}

			return cmdCommon.WriteChangesetFile(fileName, csFile, h.settings)

		default:
			h.log.Warn().Msgf("Unsupported transaction type: %s", txOut.Type)
		}

		// Workflow artifacts deletion will be handled by a background cleanup process.
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to delete some workflows: %w", errors.Join(errs...))
	}
	ui.Success("Workflows deleted successfully")
	return nil
}

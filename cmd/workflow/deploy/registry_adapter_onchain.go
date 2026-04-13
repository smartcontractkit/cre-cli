package deploy

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// onchainRegistryAdapter deploys workflows to the onchain workflow registry.
// It wraps async WRC client initialization and handles ownership linking,
// duplicate detection, and DON limit checks.
type onchainRegistryAdapter struct {
	h       *handler
	wg      sync.WaitGroup
	initErr error
}

func newOnchainRegistryAdapter(h *handler) *onchainRegistryAdapter {
	a := &onchainRegistryAdapter{h: h}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		wrc, err := h.clientFactory.NewWorkflowRegistryV2Client()
		if err != nil {
			a.initErr = fmt.Errorf("failed to create workflow registry client: %w", err)
			return
		}
		h.wrc = wrc
	}()
	return a
}

func (a *onchainRegistryAdapter) RunPreDeployChecks() error {
	h := a.h

	a.wg.Wait()
	if a.initErr != nil {
		return a.initErr
	}

	ui.Line()
	ui.Dim("Verifying ownership...")
	if h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerType == constants.WorkflowOwnerTypeMSIG {
		halt, err := h.autoLinkMSIGAndExit()
		if err != nil {
			return fmt.Errorf("failed to check/handle MSIG owner link status: %w", err)
		}
		if halt {
			return errDeployHalted
		}
	} else {
		if err := h.ensureOwnerLinkedOrFail(); err != nil {
			return err
		}
	}

	existsErr := h.workflowExists()
	if existsErr != nil {
		if existsErr.Error() == "workflow with name "+h.inputs.WorkflowName+" already exists" {
			ui.Warning(fmt.Sprintf("Workflow %s already exists", h.inputs.WorkflowName))
			ui.Dim("This will update the existing workflow.")
			if !h.inputs.SkipConfirmation {
				confirm, err := ui.Confirm("Are you sure you want to overwrite the workflow?")
				if err != nil {
					return err
				}
				if !confirm {
					return errors.New("deployment cancelled by user")
				}
			}
		} else {
			return existsErr
		}
	}

	return nil
}

func (a *onchainRegistryAdapter) Upsert() error {
	h := a.h

	if err := checkUserDonLimitBeforeDeploy(
		h.wrc,
		h.wrc,
		common.HexToAddress(h.inputs.WorkflowOwner),
		h.inputs.DonFamily,
		h.inputs.WorkflowName,
		h.inputs.KeepAlive,
		h.existingWorkflowStatus,
	); err != nil {
		return err
	}

	ui.Line()
	ui.Dim("Preparing deployment transaction...")
	if err := h.upsert(); err != nil {
		return fmt.Errorf("failed to register workflow: %w", err)
	}
	return nil
}

func (h *handler) workflowExists() error {
	workflow, err := h.wrc.GetWorkflow(common.HexToAddress(h.inputs.WorkflowOwner), h.inputs.WorkflowName, h.inputs.WorkflowTag)
	if err != nil {
		return err
	}
	if workflow.WorkflowId == [32]byte(common.Hex2Bytes(h.workflowArtifact.WorkflowID)) {
		return fmt.Errorf("workflow with id %s already exists", h.workflowArtifact.WorkflowID)
	}
	if workflow.WorkflowName == h.inputs.WorkflowName {
		status := workflow.Status
		h.existingWorkflowStatus = &status
		return fmt.Errorf("workflow with name %s already exists", h.inputs.WorkflowName)
	}
	return nil
}

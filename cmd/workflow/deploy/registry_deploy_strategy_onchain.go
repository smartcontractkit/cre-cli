package deploy

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// onchainRegistryDeployStrategy deploys workflows to the onchain workflow registry.
// It wraps async WRC client initialization and handles ownership linking,
// duplicate detection, and DON limit checks.
type onchainRegistryDeployStrategy struct {
	h       *handler
	wg      sync.WaitGroup
	initErr error
}

func newOnchainRegistryDeployStrategy(h *handler) *onchainRegistryDeployStrategy {
	a := &onchainRegistryDeployStrategy{h: h}
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

func (a *onchainRegistryDeployStrategy) RunPreDeployChecks() error {
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

	return nil
}

func (a *onchainRegistryDeployStrategy) CheckWorkflowExists() (bool, error) {
	existsErr := a.h.workflowExists()
	if existsErr == nil {
		return false, nil
	}
	if existsErr.Error() == "workflow with name "+a.h.inputs.WorkflowName+" already exists" {
		return true, nil
	}

	return false, existsErr
}

func (a *onchainRegistryDeployStrategy) Upsert() error {
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

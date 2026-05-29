package deploy

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// onchainRegistryDeployStrategy deploys workflows to the onchain workflow registry.
// It wraps async WRC client initialization and handles ownership linking,
// duplicate detection, and DON limit checks.
type onchainRegistryDeployStrategy struct {
	h       *handler
	wrc     *client.WorkflowRegistryV2Client
	onChain *settings.OnChainRegistry
	wg      sync.WaitGroup
	initErr error
}

func newOnchainRegistryDeployStrategy(h *handler) (*onchainRegistryDeployStrategy, error) {
	onChain, err := settings.AsOnChain(h.runtimeContext.ResolvedRegistry, "deploy")
	if err != nil {
		return nil, err
	}

	a := &onchainRegistryDeployStrategy{h: h, onChain: onChain}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		wrc, err := h.clientFactory.NewWorkflowRegistryV2Client(h.executionContext())
		if err != nil {
			a.initErr = fmt.Errorf("failed to create workflow registry client: %w", err)
			return
		}
		a.wrc = wrc
		h.wrc = wrc
	}()
	return a, nil
}

func waitWithContext(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *onchainRegistryDeployStrategy) RunPreDeployChecks() error {
	h := a.h

	if err := waitWithContext(a.h.executionContext(), &a.wg); err != nil {
		return err
	}
	if a.initErr != nil {
		return a.initErr
	}

	ui.Line()
	ui.Dim("Verifying ownership...")
	if h.settings.Workflow.UserWorkflowSettings.WorkflowOwnerType == constants.WorkflowOwnerTypeMSIG {
		halt, err := h.autoLinkMSIGAndExit(a.onChain)
		if err != nil {
			return fmt.Errorf("failed to check/handle MSIG owner link status: %w", err)
		}
		if halt {
			return errDeployHalted
		}
	} else {
		if err := h.ensureOwnerLinkedOrFail(a.onChain); err != nil {
			return err
		}
	}

	return nil
}

func (a *onchainRegistryDeployStrategy) CheckWorkflowExists(workflowOwner, workflowName, workflowTag, workflowID string) (bool, *uint8, error) {
	workflow, err := a.wrc.GetWorkflow(a.h.executionContext(), common.HexToAddress(workflowOwner), workflowName, workflowTag)
	if err != nil {
		return false, nil, err
	}
	if workflow.WorkflowId == [32]byte(common.Hex2Bytes(workflowID)) {
		status := workflow.Status
		return true, &status, fmt.Errorf("workflow with id %s is already registered and unchanged; re-deployment skipped: %w", workflowID, errWorkflowUnchanged)
	}
	if workflow.WorkflowName == workflowName {
		status := workflow.Status
		return true, &status, nil
	}

	return false, nil, nil
}

func (a *onchainRegistryDeployStrategy) Upsert() error {
	h := a.h

	if err := checkUserDonLimitBeforeDeploy(
		h.executionContext(),
		a.wrc,
		a.wrc,
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
	if err := h.upsert(a.onChain); err != nil {
		return fmt.Errorf("failed to register workflow: %w", err)
	}
	return nil
}

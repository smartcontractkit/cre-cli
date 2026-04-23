package activate

import (
	"fmt"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/privateregistryclient"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

type privateRegistryActivateStrategy struct {
	h   *handler
	prc *privateregistryclient.Client
}

func newPrivateRegistryActivateStrategy(h *handler) *privateRegistryActivateStrategy {
	return &privateRegistryActivateStrategy{h: h}
}

func (a *privateRegistryActivateStrategy) ensureClient() {
	if a.prc == nil {
		gql := graphqlclient.New(a.h.runtimeContext.Credentials, a.h.environmentSet, a.h.log)
		a.prc = privateregistryclient.New(gql, a.h.log)
	}
}

func (a *privateRegistryActivateStrategy) Activate() error {
	a.ensureClient()

	h := a.h
	workflowName := h.inputs.WorkflowName

	ui.Dim(fmt.Sprintf("Fetching workflow to activate... Name=%s", workflowName))

	workflow, err := a.prc.GetWorkflowByName(workflowName)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}

	if workflow.Status == privateregistryclient.WorkflowStatusActive {
		return fmt.Errorf("workflow is already active, cancelling transaction")
	}

	h.runtimeContext.Workflow.ID = workflow.WorkflowID

	ui.Dim(fmt.Sprintf("Processing activation for workflow ID %s...", workflow.WorkflowID))

	result, err := a.prc.ActivateWorkflowInRegistry(workflow.WorkflowID)
	if err != nil {
		return fmt.Errorf("failed to activate workflow in private registry: %w", err)
	}

	ui.Success("Workflow activated successfully")
	ui.Line()
	ui.Bold("Details:")
	ui.Dim(fmt.Sprintf("   Registry:         %s", h.runtimeContext.ResolvedRegistry.ID()))
	ui.Dim(fmt.Sprintf("   Workflow Name:    %s", result.WorkflowName))
	ui.Dim(fmt.Sprintf("   Workflow ID:      %s", result.WorkflowID))
	ui.Dim(fmt.Sprintf("   Status:           %s", privateregistryclient.FormatStatus(result.Status)))
	if result.Owner != "" {
		ui.Dim(fmt.Sprintf("   Owner:            %s", result.Owner))
	}

	return nil
}

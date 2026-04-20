package pause

import (
	"fmt"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/privateregistryclient"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

type privateRegistryPauseStrategy struct {
	h   *handler
	prc *privateregistryclient.Client
}

func newPrivateRegistryPauseStrategy(h *handler) *privateRegistryPauseStrategy {
	return &privateRegistryPauseStrategy{h: h}
}

func (a *privateRegistryPauseStrategy) ensureClient() {
	if a.prc == nil {
		gql := graphqlclient.New(a.h.runtimeContext.Credentials, a.h.environmentSet, a.h.log)
		a.prc = privateregistryclient.New(gql, a.h.log)
	}
}

func (a *privateRegistryPauseStrategy) Pause() error {
	a.ensureClient()

	h := a.h
	workflowName := h.inputs.WorkflowName

	ui.Dim(fmt.Sprintf("Fetching workflow to pause... Name=%s", workflowName))

	workflow, err := a.prc.GetWorkflowByName(workflowName)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}

	if workflow.Status == privateregistryclient.WorkflowStatusPaused {
		return fmt.Errorf("workflow is already paused, cancelling transaction")
	}

	h.runtimeContext.Workflow.ID = workflow.WorkflowID

	ui.Dim(fmt.Sprintf("Processing pause for workflow ID %s...", workflow.WorkflowID))

	result, err := a.prc.PauseWorkflowInRegistry(workflow.WorkflowID)
	if err != nil {
		return fmt.Errorf("failed to pause workflow in private registry: %w", err)
	}

	ui.Success("Workflow paused successfully")
	ui.Line()
	ui.Bold("Details:")
	ui.Dim(fmt.Sprintf("   Workflow Name: %s", result.WorkflowName))
	ui.Dim(fmt.Sprintf("   Workflow ID:   %s", result.WorkflowID))
	ui.Dim(fmt.Sprintf("   Status:        %s", result.Status))
	if result.Owner != "" {
		ui.Dim(fmt.Sprintf("   Owner:         %s", result.Owner))
	}

	return nil
}

package delete

import (
	"fmt"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/privateregistryclient"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

type privateRegistryDeleteStrategy struct {
	h   *handler
	prc *privateregistryclient.Client
}

func newPrivateRegistryDeleteStrategy(h *handler) *privateRegistryDeleteStrategy {
	return &privateRegistryDeleteStrategy{h: h}
}

func (a *privateRegistryDeleteStrategy) ensureClient() {
	if a.prc == nil {
		gql := graphqlclient.New(a.h.runtimeContext.Credentials, a.h.environmentSet, a.h.log)
		a.prc = privateregistryclient.New(gql, a.h.log)
	}
}

func (a *privateRegistryDeleteStrategy) FetchWorkflows() ([]WorkflowToDelete, error) {
	a.ensureClient()

	h := a.h
	workflowName := h.inputs.WorkflowName

	ui.Dim(fmt.Sprintf("Fetching workflow to delete... Name=%s", workflowName))

	workflow, err := a.prc.GetWorkflowByName(workflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	return []WorkflowToDelete{
		{
			ID:        workflow.WorkflowID,
			Owner:     workflow.Owner,
			DonFamily: workflow.DonFamily,
			Tag:       workflow.Tag,
			BinaryURL: workflow.BinaryURL,
			Status:    string(workflow.Status),
			RawID:     workflow.WorkflowID,
		},
	}, nil
}

func (a *privateRegistryDeleteStrategy) DeleteWorkflows(workflows []WorkflowToDelete) error {
	h := a.h

	for _, wf := range workflows {
		workflowID := wf.RawID.(string)
		deletedID, err := a.prc.DeleteWorkflowInRegistry(workflowID)
		if err != nil {
			h.log.Error().
				Err(err).
				Str("workflowId", workflowID).
				Msg("Failed to delete workflow")
			return fmt.Errorf("failed to delete workflow in private registry: %w", err)
		}

		ui.Success(fmt.Sprintf("Deleted workflow ID: %s", deletedID))
	}

	return nil
}

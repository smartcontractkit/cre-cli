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

func (a *privateRegistryDeleteStrategy) Delete() error {
	a.ensureClient()

	h := a.h
	workflowName := h.inputs.WorkflowName

	ui.Dim(fmt.Sprintf("Fetching workflow to delete... Name=%s", workflowName))

	workflow, err := a.prc.GetWorkflowByName(workflowName)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}

	h.runtimeContext.Workflow.ID = workflow.WorkflowID

	ui.Bold(fmt.Sprintf("Found 1 workflow(s) to delete for name: %s", workflowName))
	ui.Print("   1. Workflow")
	ui.Dim(fmt.Sprintf("      ID:              %s", workflow.WorkflowID))
	ui.Dim(fmt.Sprintf("      Owner:           %s", workflow.Owner))
	ui.Dim(fmt.Sprintf("      DON Family:      %s", workflow.DonFamily))
	ui.Dim(fmt.Sprintf("      Tag:             %s", workflow.Tag))
	ui.Dim(fmt.Sprintf("      Binary URL:      %s", workflow.BinaryUrl))
	ui.Dim(fmt.Sprintf("      Workflow Status: %s", workflow.Status))
	ui.Line()

	shouldDeleteWorkflow, err := h.shouldDeleteWorkflow(h.inputs.SkipConfirmation, workflowName)
	if err != nil {
		return err
	}
	if !shouldDeleteWorkflow {
		ui.Warning("Workflow deletion canceled")
		return nil
	}

	ui.Dim("Deleting 1 workflow(s)...")

	deletedID, err := a.prc.DeleteWorkflowInRegistry(workflow.WorkflowID)
	if err != nil {
		h.log.Error().
			Err(err).
			Str("workflowId", workflow.WorkflowID).
			Msg("Failed to delete workflow")
		return fmt.Errorf("failed to delete workflow in private registry: %w", err)
	}

	ui.Success(fmt.Sprintf("Deleted workflow ID: %s", deletedID))
	ui.Success("Workflows deleted successfully")

	return nil
}

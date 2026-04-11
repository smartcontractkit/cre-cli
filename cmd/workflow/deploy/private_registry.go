package deploy

import (
	"fmt"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/privateregistryclient"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// runPrivateRegistryDeploy handles deployment to the private workflow registry
// via GraphQL. Ownership linking and onchain prechecks are not applicable.
func (h *handler) runPrivateRegistryDeploy() error {
	if !h.validated {
		return fmt.Errorf("handler inputs not validated")
	}

	input := h.buildPrivateRegistryInput()

	gql := graphqlclient.New(h.credentials, h.environmentSet, h.log)
	prc := privateregistryclient.New(gql, h.log)

	ui.Line()
	ui.Dim(fmt.Sprintf("Registering workflow in private registry (workflowID: %s)...", input.WorkflowID))

	result, err := prc.UpsertWorkflowInRegistry(input)
	if err != nil {
		return fmt.Errorf("failed to register workflow in private registry: %w", err)
	}

	ui.Success("Workflow registered in private registry")
	ui.Line()
	ui.Bold("Details:")
	ui.Dim(fmt.Sprintf("   Workflow Name: %s", result.WorkflowName))
	ui.Dim(fmt.Sprintf("   Workflow ID:   %s", result.WorkflowID))
	ui.Dim(fmt.Sprintf("   Status:        %s", result.Status))
	ui.Dim(fmt.Sprintf("   Binary URL:    %s", result.BinaryURL))
	if result.ConfigURL != "" {
		ui.Dim(fmt.Sprintf("   Config URL:    %s", result.ConfigURL))
	}
	if result.Owner != "" {
		ui.Dim(fmt.Sprintf("   Owner:         %s", result.Owner))
	}

	return nil
}

func (h *handler) buildPrivateRegistryInput() privateregistryclient.WorkflowInRegistryInput {
	input := privateregistryclient.WorkflowInRegistryInput{
		WorkflowID:   h.workflowArtifact.WorkflowID,
		Status:       privateregistryclient.WorkflowStatusActive,
		WorkflowName: h.inputs.WorkflowName,
		BinaryURL:    h.inputs.BinaryURL,
		DonFamily:    h.inputs.DonFamily,
	}

	if configURL := h.inputs.ResolveConfigURL(""); configURL != "" {
		input.ConfigURL = &configURL
	}

	if h.inputs.WorkflowTag != "" {
		tag := h.inputs.WorkflowTag
		input.Tag = &tag
	}

	return input
}

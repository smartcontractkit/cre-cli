package deploy

import (
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/privateregistryclient"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// privateRegistryDeployStrategy deploys workflows to the private workflow registry
// via GraphQL. Ownership linking and onchain prechecks are not applicable.
type privateRegistryDeployStrategy struct {
	h   *handler
	prc *privateregistryclient.Client
}

func newPrivateRegistryDeployStrategy(h *handler) *privateRegistryDeployStrategy {
	return &privateRegistryDeployStrategy{h: h}
}

func (a *privateRegistryDeployStrategy) ensureClient() {
	if a.prc == nil {
		gql := graphqlclient.New(a.h.credentials, a.h.environmentSet, a.h.log)
		a.prc = privateregistryclient.New(gql, a.h.log)
	}
}

func (a *privateRegistryDeployStrategy) RunPreDeployChecks() error {
	return nil
}

func (a *privateRegistryDeployStrategy) CheckWorkflowExists() (bool, error) {
	a.ensureClient()

	_, err := a.prc.GetWorkflowByName(a.h.inputs.WorkflowName)
	if err == nil {
		return true, nil
	}
	if isWorkflowNotFoundError(err) {
		return false, nil
	}

	return false, err
}

func (a *privateRegistryDeployStrategy) Upsert() error {
	a.ensureClient()

	h := a.h
	input := h.buildPrivateRegistryInput()

	ui.Line()
	ui.Dim(fmt.Sprintf("Registering workflow in private registry (workflowID: %s)...", input.WorkflowID))

	result, err := a.prc.UpsertWorkflowInRegistry(input)
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

func (h *handler) buildPrivateRegistryInput() privateregistryclient.OffchainWorkflowInput {
	input := privateregistryclient.OffchainWorkflowInput{
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

func isWorkflowNotFoundError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found")
}

package deploy

import (
	"fmt"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/client/privateregistryclient"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// privateRegistryAdapter deploys workflows to the private workflow registry
// via GraphQL. Ownership linking and onchain prechecks are not applicable.
type privateRegistryAdapter struct {
	h   *handler
	prc *privateregistryclient.Client
}

func newPrivateRegistryAdapter(h *handler) *privateRegistryAdapter {
	return &privateRegistryAdapter{h: h}
}

func (a *privateRegistryAdapter) ensureClient() {
	if a.prc == nil {
		gql := graphqlclient.New(a.h.credentials, a.h.environmentSet, a.h.log)
		a.prc = privateregistryclient.New(gql, a.h.log)
	}
}

func (a *privateRegistryAdapter) RunPreDeployChecks() error {
	if !a.h.validated {
		return fmt.Errorf("handler inputs not validated")
	}
	// TODO: check if workflow already exists in private registry and confirm update
	return nil
}

func (a *privateRegistryAdapter) Upsert() error {
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

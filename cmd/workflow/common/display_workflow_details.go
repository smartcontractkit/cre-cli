package common

import (
	"fmt"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

func DisplayWorkflowDetails(
	cfg *settings.Settings,
	runtimeContext *runtime.Context,
	action,
	workflowName,
	ownerAddress string,
) {
	displayOwnerAddress := ownerAddress
	if runtimeContext.ResolvedRegistry.Type() == settings.RegistryTypeOffChain {
		displayOwnerAddress += " (derived)"
	}

	ui.Line()
	ui.Title(fmt.Sprintf("%s Workflow: %s", action, workflowName))
	ui.Dim(fmt.Sprintf("Registry:      %s", runtimeContext.ResolvedRegistry.ID()))
	ui.Dim(fmt.Sprintf("DON Family:    %s", runtimeContext.ResolvedRegistry.DonFamily()))
	ui.Dim(fmt.Sprintf("Target:        %s", cfg.User.TargetName))
	ui.Dim(fmt.Sprintf("Owner Address: %s", displayOwnerAddress))
	ui.Line()
}

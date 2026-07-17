package workflowresolve

import (
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/settings"
)

// ResolveWorkflowOwnerAddress returns the effective workflow owner for platform
// lookups. For private/off-chain registry deploys the derived workflow owner from
// the runtime context is used. For on-chain deploys the configured
// workflow-owner-address from the selected target is used.
func ResolveWorkflowOwnerAddress(
	s *settings.Settings,
	resolvedRegistry settings.ResolvedRegistry,
	derivedOwner string,
) (string, error) {
	if resolvedRegistry != nil && resolvedRegistry.Type() == settings.RegistryTypeOffChain {
		owner := strings.TrimSpace(derivedOwner)
		if owner == "" {
			return "", fmt.Errorf("derived workflow owner is not available; ensure authentication succeeded")
		}
		return owner, nil
	}

	if s == nil {
		return "", nil
	}
	return strings.TrimSpace(s.Workflow.UserWorkflowSettings.WorkflowOwnerAddress), nil
}

package list

import (
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

func omitDeleted(rows []Workflow) []Workflow {
	out := make([]Workflow, 0, len(rows))
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Status), "DELETED") {
			continue
		}
		out = append(out, r)
	}
	return out
}

func printWorkflowTable(rows []Workflow, registries []*tenantctx.Registry, afterRegistryFilter int, includeDeleted bool) {
	ui.Line()
	if len(rows) == 0 {
		if afterRegistryFilter > 0 && !includeDeleted {
			ui.Warning("No workflows found (excluding deleted). Use --include-deleted to list them.")
		} else {
			ui.Warning("No workflows found")
		}
		ui.Line()
		return
	}

	ui.Bold("Workflows")
	ui.Line()

	for i, r := range rows {
		matchedReg := resolveWorkflowRegistry(r.WorkflowSource, registries)
		regIDCol := formatRegistryIDFromResolved(r.WorkflowSource, matchedReg)
		ui.Bold(fmt.Sprintf("%d. %s", i+1, r.Name))
		ui.Dim(fmt.Sprintf("   Workflow ID:  %s", r.WorkflowID))
		ui.Dim(fmt.Sprintf("   Owner:        %s", r.OwnerAddress))
		ui.Dim(fmt.Sprintf("   Status:       %s", r.Status))
		ui.Dim(fmt.Sprintf("   Registry:     %s", regIDCol))
		if matchedReg != nil && registryEligibleForContractRows(matchedReg) && matchedReg.Address != nil {
			ui.Dim(fmt.Sprintf("   Address:      %s", strings.TrimSpace(*matchedReg.Address)))
		} else if _, addr, ok := parseContractWorkflowSource(r.WorkflowSource); ok && strings.TrimSpace(addr) != "" {
			ui.Dim(fmt.Sprintf("   Address:      %s", strings.TrimSpace(addr)))
		}
		ui.Line()
	}
}

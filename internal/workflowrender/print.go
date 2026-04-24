package workflowrender

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// workflowJSON is the JSON representation of a workflow for stdout output.
type workflowJSON struct {
	Name            string `json:"name"`
	WorkflowID      string `json:"workflowId"`
	OwnerAddress    string `json:"ownerAddress"`
	Status          string `json:"status"`
	Registry        string `json:"registry"`
	ContractAddress string `json:"contractAddress,omitempty"`
}

func buildWorkflowJSON(rows []Workflow, registries []*tenantctx.Registry) []workflowJSON {
	out := make([]workflowJSON, 0, len(rows))
	for _, r := range rows {
		matched := ResolveWorkflowRegistry(r.WorkflowSource, registries)
		entry := workflowJSON{
			Name:         r.Name,
			WorkflowID:   r.WorkflowID,
			OwnerAddress: r.OwnerAddress,
			Status:       r.Status,
			Registry:     RegistryIDOrSource(r.WorkflowSource, matched),
		}
		if matched != nil && RegistryEligibleForContractRows(matched) && matched.Address != nil {
			entry.ContractAddress = strings.TrimSpace(*matched.Address)
		} else if _, addr, ok := ParseContractWorkflowSource(r.WorkflowSource); ok && strings.TrimSpace(addr) != "" {
			entry.ContractAddress = strings.TrimSpace(addr)
		}
		out = append(out, entry)
	}
	return out
}

// PrintWorkflowsJSON marshals workflows as an indented JSON array and writes it to stdout.
func PrintWorkflowsJSON(rows []Workflow, registries []*tenantctx.Registry) error {
	data, err := json.MarshalIndent(buildWorkflowJSON(rows, registries), "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// OmitDeleted returns rows whose status is not "DELETED" (case-insensitive).
func OmitDeleted(rows []Workflow) []Workflow {
	out := make([]Workflow, 0, len(rows))
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Status), "DELETED") {
			continue
		}
		out = append(out, r)
	}
	return out
}

// TableOptions controls the empty-state hint printed by PrintWorkflowTable.
type TableOptions struct {
	// CountBeforeDeletedFilter is the number of rows after any registry/search
	// filtering but before DELETED rows were removed. When it is > 0 and
	// IncludeDeleted is false, the empty-state message hints at --include-deleted.
	CountBeforeDeletedFilter int
	// IncludeDeleted indicates whether the caller is already showing DELETED rows.
	IncludeDeleted bool
}

// PrintWorkflowTable renders the workflow rows as a bulleted table using the
// shared UI helpers.
func PrintWorkflowTable(rows []Workflow, registries []*tenantctx.Registry, opts TableOptions) {
	ui.Line()
	if len(rows) == 0 {
		if opts.CountBeforeDeletedFilter > 0 && !opts.IncludeDeleted {
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
		matchedReg := ResolveWorkflowRegistry(r.WorkflowSource, registries)
		regIDCol := RegistryIDOrSource(r.WorkflowSource, matchedReg)
		ui.Bold(fmt.Sprintf("%d. %s", i+1, r.Name))
		ui.Dim(fmt.Sprintf("   Workflow ID:  %s", r.WorkflowID))
		ui.Dim(fmt.Sprintf("   Owner:        %s", r.OwnerAddress))
		ui.Dim(fmt.Sprintf("   Status:       %s", r.Status))
		ui.Dim(fmt.Sprintf("   Registry:     %s", regIDCol))
		if matchedReg != nil && RegistryEligibleForContractRows(matchedReg) && matchedReg.Address != nil {
			ui.Dim(fmt.Sprintf("   Address:      %s", strings.TrimSpace(*matchedReg.Address)))
		} else if _, addr, ok := ParseContractWorkflowSource(r.WorkflowSource); ok && strings.TrimSpace(addr) != "" {
			ui.Dim(fmt.Sprintf("   Address:      %s", strings.TrimSpace(addr)))
		}
		ui.Line()
	}
}

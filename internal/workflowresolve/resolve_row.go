package workflowresolve

import (
	"context"
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// RowResolveOptions controls how ResolveWorkflowRow narrows candidates.
type RowResolveOptions struct {
	// OwnerFilter scopes name search to workflows owned by this address.
	OwnerFilter string
	// RegistryFilter scopes candidates to the registry with this ID.
	RegistryFilter string
	// Registries is the tenant registry list, used for RegistryFilter and picker labels.
	Registries []*tenantctx.Registry
	// NonInteractive fails instead of prompting when the selection is ambiguous.
	NonInteractive bool
	// CandidateFilter further restricts eligible rows (e.g. ACTIVE-only for pause);
	// nil means every non-deleted row is eligible.
	CandidateFilter func(Workflow) bool
	// PickerTitle is the title shown on the interactive picker.
	PickerTitle string
	// Pick selects one row among several candidates; nil uses the default interactive picker.
	Pick func(candidates []Workflow, title string, registries []*tenantctx.Registry) (*Workflow, error)
}

// ResolveWorkflowRow resolves a workflow reference to a single platform workflow row.
// The reference can be empty (interactive picker over all workflows), an on-chain
// workflow ID (64-char hex), or a workflow name. Deleted workflows are excluded.
func ResolveWorkflowRow(ctx context.Context, wdc WorkflowLookup, arg string, opts RowResolveOptions) (*Workflow, error) {
	arg = strings.TrimSpace(arg)

	if err := opts.checkRegistryFilter(); err != nil {
		return nil, err
	}

	switch {
	case arg == "":
		return resolveInteractively(ctx, wdc, opts)
	case LooksLikeWorkflowID(arg):
		return resolveRowByWorkflowID(ctx, wdc, arg, opts)
	default:
		return resolveRowByName(ctx, wdc, arg, opts)
	}
}

func resolveInteractively(ctx context.Context, wdc WorkflowLookup, opts RowResolveOptions) (*Workflow, error) {
	if opts.NonInteractive {
		return nil, fmt.Errorf("a workflow name or ID is required in non-interactive mode")
	}

	spinner := ui.NewSpinner()
	spinner.Start("Listing workflows...")
	rows, err := wdc.ListAll(ctx, workflowdataclient.DefaultPageSize)
	spinner.Stop()
	if err != nil {
		return nil, fmt.Errorf("listing workflows: %w", err)
	}

	candidates := narrowRows(OmitDeleted(rows), opts)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no eligible workflows found")
	}
	return pickRow(candidates, opts)
}

func resolveRowByWorkflowID(ctx context.Context, wdc WorkflowLookup, workflowID string, opts RowResolveOptions) (*Workflow, error) {
	spinner := ui.NewSpinner()
	spinner.Start(fmt.Sprintf("Resolving workflow ID %q...", workflowID))
	rows, err := wdc.ListAll(ctx, workflowdataclient.DefaultPageSize)
	spinner.Stop()
	if err != nil {
		return nil, fmt.Errorf("resolving workflow ID %q: %w", workflowID, err)
	}

	for _, r := range rows {
		if strings.EqualFold(r.WorkflowID, workflowID) {
			if !eligible(r, opts) {
				return nil, fmt.Errorf("workflow %q (name %q, status %s) is not eligible for this operation", workflowID, r.Name, r.Status)
			}
			row := r
			return &row, nil
		}
	}
	return nil, fmt.Errorf("no workflow found with ID %q", workflowID)
}

func resolveRowByName(ctx context.Context, wdc WorkflowLookup, name string, opts RowResolveOptions) (*Workflow, error) {
	spinner := ui.NewSpinner()
	spinner.Start(fmt.Sprintf("Resolving workflow %q...", name))
	rows, err := wdc.SearchByName(ctx, name, workflowdataclient.DefaultPageSize, opts.OwnerFilter)
	spinner.Stop()
	if err != nil {
		return nil, fmt.Errorf("resolving workflow name %q: %w", name, err)
	}

	var matches []Workflow
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Name), name) {
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no workflow found with name %q", name)
	}

	candidates := narrowRows(OmitDeleted(matches), opts)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("workflow %q was found but none of its deployments are eligible (status must not be DELETED and must pass the operation's filter)", name)
	}
	if len(candidates) == 1 {
		return &candidates[0], nil
	}
	if opts.NonInteractive {
		return nil, fmt.Errorf("multiple workflows named %q found; provide the workflow ID instead (see 'cre workflow list'), or scope with --registry", name)
	}
	return pickRow(candidates, opts)
}

// narrowRows applies the registry filter and candidate filter to rows.
func narrowRows(rows []Workflow, opts RowResolveOptions) []Workflow {
	if opts.RegistryFilter != "" {
		reg := FindRegistry(opts.Registries, opts.RegistryFilter)
		rows = FilterRowsByRegistry(rows, reg, opts.Registries)
	}
	if opts.CandidateFilter == nil {
		return rows
	}
	out := make([]Workflow, 0, len(rows))
	for _, r := range rows {
		if opts.CandidateFilter(r) {
			out = append(out, r)
		}
	}
	return out
}

func eligible(row Workflow, opts RowResolveOptions) bool {
	if opts.RegistryFilter != "" {
		reg := FindRegistry(opts.Registries, opts.RegistryFilter)
		if len(FilterRowsByRegistry([]Workflow{row}, reg, opts.Registries)) == 0 {
			return false
		}
	}
	return opts.CandidateFilter == nil || opts.CandidateFilter(row)
}

func (opts RowResolveOptions) checkRegistryFilter() error {
	if opts.RegistryFilter == "" {
		return nil
	}
	if FindRegistry(opts.Registries, opts.RegistryFilter) == nil {
		return fmt.Errorf("registry %q not found in user context; available: [%s]",
			opts.RegistryFilter, AvailableRegistryIDs(opts.Registries))
	}
	return nil
}

func pickRow(candidates []Workflow, opts RowResolveOptions) (*Workflow, error) {
	if opts.Pick != nil {
		return opts.Pick(candidates, opts.PickerTitle, opts.Registries)
	}
	return PickWorkflow(candidates, opts.PickerTitle, opts.Registries)
}

// PickWorkflow renders an interactive picker over candidate workflow rows.
func PickWorkflow(candidates []Workflow, title string, registries []*tenantctx.Registry) (*Workflow, error) {
	if title == "" {
		title = "Select a workflow"
	}
	options := make([]ui.SelectOption[int], len(candidates))
	for i, c := range candidates {
		matched := ResolveWorkflowRegistry(c.WorkflowSource, registries)
		options[i] = ui.SelectOption[int]{
			Label: fmt.Sprintf("%s  (registry: %s, status: %s, id: %s)", c.Name, RegistryIDOrSource(c.WorkflowSource, matched), c.Status, ShortID(c.WorkflowID)),
			Value: i,
		}
	}
	idx, err := ui.Select[int](title, options)
	if err != nil {
		return nil, err
	}
	return &candidates[idx], nil
}

// ShortID truncates a workflow ID for compact display.
func ShortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:6] + "…" + id[len(id)-4:]
}

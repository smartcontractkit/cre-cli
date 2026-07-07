package workflowresolve

import (
	"context"
	"fmt"
	"strings"

	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// WorkflowLookup lists workflows for name and WorkflowId resolution.
type WorkflowLookup interface {
	ListAll(ctx context.Context, pageSize int) ([]workflowdataclient.Workflow, error)
	SearchByName(ctx context.Context, name string, pageSize int) ([]workflowdataclient.Workflow, error)
}

// ResolveOptions controls resolution behaviour for ambiguous workflow names.
type ResolveOptions struct {
	// NonInteractive suppresses warnings when falling back to a non-ACTIVE workflow.
	NonInteractive bool
}

// ResolveWorkflowUUID returns the platform UUID for a workflow name or on-chain WorkflowId.
func ResolveWorkflowUUID(ctx context.Context, wdc WorkflowLookup, arg string, opts ResolveOptions) (string, error) {
	if LooksLikeWorkflowID(arg) {
		return resolveByWorkflowID(ctx, wdc, arg)
	}
	return resolveByName(ctx, wdc, arg, opts)
}

func resolveByWorkflowID(ctx context.Context, wdc WorkflowLookup, workflowID string) (string, error) {
	spinner := ui.NewSpinner()
	spinner.Start(fmt.Sprintf("Resolving workflow ID %q...", workflowID))
	rows, err := wdc.ListAll(ctx, workflowdataclient.DefaultPageSize)
	spinner.Stop()
	if err != nil {
		return "", fmt.Errorf("resolving workflow ID %q: %w", workflowID, err)
	}

	for _, r := range rows {
		if strings.EqualFold(r.WorkflowID, workflowID) {
			if r.UUID == "" {
				return "", fmt.Errorf("workflow with ID %q found but has no platform UUID", workflowID)
			}
			return r.UUID, nil
		}
	}
	return "", fmt.Errorf("no workflow found with ID %q", workflowID)
}

func resolveByName(ctx context.Context, wdc WorkflowLookup, name string, opts ResolveOptions) (string, error) {
	spinner := ui.NewSpinner()
	spinner.Start(fmt.Sprintf("Resolving workflow %q...", name))
	rows, err := wdc.SearchByName(ctx, name, workflowdataclient.DefaultPageSize)
	spinner.Stop()
	if err != nil {
		return "", fmt.Errorf("resolving workflow name %q: %w", name, err)
	}

	var matches []workflowdataclient.Workflow
	for _, r := range rows {
		if strings.EqualFold(strings.TrimSpace(r.Name), name) {
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no workflow found with name %q", name)
	}

	var active []workflowdataclient.Workflow
	for _, r := range matches {
		if strings.EqualFold(r.Status, "ACTIVE") {
			active = append(active, r)
		}
	}
	if len(active) == 1 {
		return active[0].UUID, nil
	}
	if len(active) > 1 {
		return "", fmt.Errorf("multiple ACTIVE workflows named %q found; provide the workflow ID instead", name)
	}

	if !opts.NonInteractive {
		ui.Warning(fmt.Sprintf("No ACTIVE deployment for workflow %q; using the first match (status: %s)", name, matches[0].Status))
	}
	if matches[0].UUID == "" {
		return "", fmt.Errorf("workflow %q resolved but has no platform UUID", name)
	}
	return matches[0].UUID, nil
}

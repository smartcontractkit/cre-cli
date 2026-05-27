package remove

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/templateconfig"
	"github.com/smartcontractkit/cre-cli/internal/templaterepo"
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

type handler struct {
	log *zerolog.Logger
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <owner/repo>...",
		Short:   "Removes a template repository source",
		Long:    `Removes one or more template repository sources from ~/.cre/template.yaml. The ref portion is optional and ignored during matching.`,
		Args:    cobra.MinimumNArgs(1),
		Example: "cre templates remove smartcontractkit/cre-templates myorg/my-templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			h := &handler{log: runtimeContext.Logger}
			return h.Execute(args)
		},
	}
}

func (h *handler) Execute(repos []string) error {
	if err := templateconfig.EnsureDefaultConfig(h.log); err != nil {
		return fmt.Errorf("failed to initialize template config: %w", err)
	}

	existing := templateconfig.LoadTemplateSources(h.log)

	// Build lookup of repos to remove (match on owner/repo, ignore ref)
	toRemove := make(map[string]bool, len(repos))
	for _, repoStr := range repos {
		source, err := templateconfig.ParseRepoString(repoStr)
		if err != nil {
			return fmt.Errorf("invalid repo format %q: %w", repoStr, err)
		}
		toRemove[source.Owner+"/"+source.Repo] = true
	}

	var remaining []templaterepo.RepoSource
	var removed []templaterepo.RepoSource
	for _, s := range existing {
		key := s.Owner + "/" + s.Repo
		if toRemove[key] {
			removed = append(removed, s)
			delete(toRemove, key)
		} else {
			remaining = append(remaining, s)
		}
	}

	// Warn about repos that weren't found
	for key := range toRemove {
		ui.Warning(fmt.Sprintf("Repository %s is not configured, skipping", key))
	}

	if len(removed) == 0 {
		return nil
	}

	if err := templateconfig.SaveTemplateSources(remaining); err != nil {
		return fmt.Errorf("failed to save template config: %w", err)
	}

	// Invalidate cache for removed sources
	invalidateCache(h.log, removed)

	ui.Line()
	for _, s := range removed {
		ui.Success(fmt.Sprintf("Removed %s", s.String()))
	}
	ui.Line()
	if len(remaining) > 0 {
		ui.Dim("Remaining repositories:")
		for _, s := range remaining {
			fmt.Printf("  - %s\n", s.String())
		}
	} else {
		ui.Dim("No template repositories configured")
		ui.Dim("Add one with: cre templates add owner/repo[@ref]")
	}
	ui.Line()

	return nil
}

func invalidateCache(logger *zerolog.Logger, sources []templaterepo.RepoSource) {
	cache, err := templaterepo.NewCache(logger)
	if err != nil {
		logger.Debug().Err(err).Msg("Could not open cache for invalidation")
		return
	}
	for _, s := range sources {
		cache.InvalidateTemplateList(s)
	}
}

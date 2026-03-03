package add

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
		Use:     "add <owner/repo[@ref]>...",
		Short:   "Adds a template repository source",
		Long:    `Adds one or more template repository sources to ~/.cre/template.yaml. These repositories are used by cre init to discover available templates.`,
		Args:    cobra.MinimumNArgs(1),
		Example: "cre templates add smartcontractkit/cre-templates@main myorg/my-templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			h := &handler{log: runtimeContext.Logger}
			return h.Execute(args)
		},
	}
}

func (h *handler) Execute(repos []string) error {
	// Parse all repo strings first
	var newSources []templaterepo.RepoSource
	for _, repoStr := range repos {
		source, err := templateconfig.ParseRepoString(repoStr)
		if err != nil {
			return fmt.Errorf("invalid repo format %q: %w", repoStr, err)
		}
		newSources = append(newSources, source)
	}

	if err := templateconfig.EnsureDefaultConfig(h.log); err != nil {
		return fmt.Errorf("failed to initialize template config: %w", err)
	}

	existing := templateconfig.LoadTemplateSources(h.log)

	// Deduplicate: skip repos already configured
	added := make([]templaterepo.RepoSource, 0, len(newSources))
	for _, ns := range newSources {
		alreadyExists := false
		for _, es := range existing {
			if es.Owner == ns.Owner && es.Repo == ns.Repo {
				ui.Warning(fmt.Sprintf("Repository %s/%s is already configured, skipping", ns.Owner, ns.Repo))
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			added = append(added, ns)
		}
	}

	if len(added) == 0 {
		return nil
	}

	updated := append(existing, added...)

	if err := templateconfig.SaveTemplateSources(updated); err != nil {
		return fmt.Errorf("failed to save template config: %w", err)
	}

	// Invalidate cache for newly added sources so cre init fetches fresh data
	invalidateCache(h.log, added)

	ui.Line()
	for _, s := range added {
		ui.Success(fmt.Sprintf("Added %s", s.String()))
	}
	ui.Line()
	ui.Dim("Configured repositories:")
	for _, s := range updated {
		fmt.Printf("  - %s\n", s.String())
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

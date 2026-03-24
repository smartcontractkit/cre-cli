package list

import (
	"encoding/json"
	"fmt"
	"strings"

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
	var refresh bool

	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists available templates",
		Long:  `Fetches and displays all templates available from configured repository sources. These can be installed with cre init.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := &handler{log: runtimeContext.Logger}
			return h.Execute(refresh, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&refresh, "refresh", false, "Bypass cache and fetch fresh data")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output template list as JSON")

	return cmd
}

func (h *handler) Execute(refresh bool, jsonOutput bool) error {
	if err := templateconfig.EnsureDefaultConfig(h.log); err != nil {
		return fmt.Errorf("failed to initialize template config: %w", err)
	}

	sources := templateconfig.LoadTemplateSources(h.log)

	if len(sources) == 0 {
		ui.Line()
		ui.Warning("No template repositories configured")
		ui.Dim("Add one with: cre templates add owner/repo[@ref]")
		ui.Line()
		return nil
	}

	registry, err := templaterepo.NewRegistry(h.log, sources)
	if err != nil {
		return fmt.Errorf("failed to create template registry: %w", err)
	}

	spinner := ui.NewSpinner()
	spinner.Start("Fetching templates...")
	templates, err := registry.ListTemplates(refresh)
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to list templates: %w", err)
	}

	if len(templates) == 0 {
		ui.Line()
		ui.Warning("No templates found in configured repositories")
		ui.Line()
		return nil
	}

	if jsonOutput {
		var filtered []templaterepo.TemplateSummary
		for _, t := range templates {
			if t.Category == templaterepo.CategoryWorkflow {
				filtered = append(filtered, t)
			}
		}
		data, err := json.MarshalIndent(filtered, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal templates: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	ui.Line()
	ui.Title("Available Templates")
	ui.Line()

	for _, t := range templates {
		// Only show workflow templates
		if t.Category != templaterepo.CategoryWorkflow {
			continue
		}

		title := t.Title
		if title == "" {
			title = t.Name
		}

		ui.Bold(fmt.Sprintf("  %s", title))

		details := fmt.Sprintf("    ID: %s", t.Name)
		if t.Language != "" {
			details += fmt.Sprintf("  |  Language: %s", t.Language)
		}
		ui.Dim(details)

		if t.Description != "" {
			ui.Dim(fmt.Sprintf("    %s", t.Description))
		}

		if len(t.Solutions) > 0 {
			ui.Dim(fmt.Sprintf("    Solutions: %s", strings.Join(t.Solutions, ", ")))
		}
		if len(t.Capabilities) > 0 {
			ui.Dim(fmt.Sprintf("    Capabilities: %s", strings.Join(t.Capabilities, ", ")))
		}
		if len(t.Tags) > 0 {
			ui.Dim(fmt.Sprintf("    Tags: %s", strings.Join(t.Tags, ", ")))
		}
		if len(t.Networks) > 0 {
			ui.Dim(fmt.Sprintf("    Networks: %s", strings.Join(t.Networks, ", ")))
		}

		ui.Line()
	}

	ui.Dim("Install a template with:")
	ui.Command("  cre init --template=<id>")
	ui.Line()
	ui.Dim("If a template requires Networks, provide them with:")
	ui.Command("  cre init --template=<id> --rpc-url=\"<chain-name>=<url>\"")
	ui.Line()

	return nil
}

package templaterepo

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

//go:embed builtin/hello-world-go/* builtin/hello-world-go/**/*
var builtinGoFS embed.FS

//go:embed builtin/hello-world-ts/* builtin/hello-world-ts/**/*
var builtinTSFS embed.FS

// BuiltInGoTemplate is the embedded hello-world Go template that is always available.
var BuiltInGoTemplate = TemplateSummary{
	TemplateMetadata: TemplateMetadata{
		Kind:        "building-block",
		Name:        "hello-world-go",
		Title:       "Hello World (Go)",
		Description: "A minimal cron-triggered workflow to get started from scratch",
		Language:    "go",
		Category:    "getting-started",
		Author:      "Chainlink",
		License:     "MIT",
		Tags:        []string{"cron", "starter", "minimal"},
	},
	Path:    "builtin/hello-world-go",
	BuiltIn: true,
}

// BuiltInTSTemplate is the embedded hello-world TypeScript template that is always available.
var BuiltInTSTemplate = TemplateSummary{
	TemplateMetadata: TemplateMetadata{
		Kind:        "building-block",
		Name:        "hello-world-ts",
		Title:       "Hello World (TypeScript)",
		Description: "A minimal cron-triggered workflow to get started from scratch",
		Language:    "typescript",
		Category:    "getting-started",
		Author:      "Chainlink",
		License:     "MIT",
		Tags:        []string{"cron", "starter", "minimal"},
	},
	Path:    "builtin/hello-world-ts",
	BuiltIn: true,
}

// BuiltInTemplates returns all built-in templates.
func BuiltInTemplates() []TemplateSummary {
	return []TemplateSummary{BuiltInGoTemplate, BuiltInTSTemplate}
}

// ScaffoldBuiltIn extracts the appropriate embedded hello-world template to destDir,
// renaming the workflow directory to the user's workflow name.
func ScaffoldBuiltIn(logger *zerolog.Logger, templateName, destDir, workflowName string) error {
	var embeddedFS embed.FS
	var templateRoot string

	switch templateName {
	case "hello-world-ts":
		embeddedFS = builtinTSFS
		templateRoot = "builtin/hello-world-ts"
	default:
		embeddedFS = builtinGoFS
		templateRoot = "builtin/hello-world-go"
	}

	err := fs.WalkDir(embeddedFS, templateRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Get path relative to the template root
		relPath, relErr := filepath.Rel(templateRoot, path)
		if relErr != nil {
			return fmt.Errorf("failed to compute relative path for %s: %w", path, relErr)
		}
		if relPath == "." {
			return nil
		}

		// Rename the "workflow" directory to the user's workflow name
		targetRel := relPath
		if relPath == "workflow" || filepath.Dir(relPath) == "workflow" {
			targetRel = filepath.Join(workflowName, relPath[len("workflow"):])
			if targetRel == workflowName+"/" {
				targetRel = workflowName
			}
		}
		// Handle nested paths under workflow/
		if len(relPath) > len("workflow/") && relPath[:len("workflow/")] == "workflow/" {
			targetRel = filepath.Join(workflowName, relPath[len("workflow/"):])
		}

		targetPath := filepath.Join(destDir, targetRel)

		if d.IsDir() {
			logger.Debug().Msgf("Extracting dir: %s -> %s", path, targetPath)
			return os.MkdirAll(targetPath, 0755)
		}

		// Read from embed
		content, readErr := embeddedFS.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, readErr)
		}

		// Write to disk
		if mkErr := os.MkdirAll(filepath.Dir(targetPath), 0755); mkErr != nil {
			return fmt.Errorf("failed to create directory: %w", mkErr)
		}

		logger.Debug().Msgf("Extracting file: %s -> %s", path, targetPath)
		return os.WriteFile(targetPath, content, 0600) //nolint:gosec // template files need to be readable
	})

	return err
}

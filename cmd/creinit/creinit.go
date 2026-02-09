package creinit

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/internal/config"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/templaterepo"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

var chainlinkTheme = ui.ChainlinkTheme()

type Inputs struct {
	ProjectName  string `validate:"omitempty,project_name" cli:"project-name"`
	TemplateName string `validate:"omitempty" cli:"template"`
	WorkflowName string `validate:"omitempty,workflow_name" cli:"workflow-name"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var initCmd = &cobra.Command{
		Use:     "init",
		Aliases: []string{"new"},
		Short:   "Initialize a new cre project (recommended starting point)",
		Long: `Initialize a new CRE project or add a workflow to an existing one.

This sets up the project structure, configuration, and starter files so you can
build, test, and deploy workflows quickly.

Templates are fetched dynamically from GitHub repositories.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := newHandler(runtimeContext)

			inputs, err := handler.ResolveInputs(runtimeContext.Viper)
			if err != nil {
				return err
			}
			err = handler.ValidateInputs(inputs)
			if err != nil {
				return err
			}
			return handler.Execute(inputs)
		},
	}

	initCmd.Flags().StringP("project-name", "p", "", "Name for the new project")
	initCmd.Flags().StringP("workflow-name", "w", "", "Name for the new workflow")
	initCmd.Flags().StringP("template", "t", "", "Name of the template to use (e.g., kv-store-go)")
	initCmd.Flags().Bool("refresh", false, "Bypass template cache and fetch fresh data")

	// Deprecated: --template-id is kept for backwards compatibility, maps to hello-world-go
	initCmd.Flags().Uint32("template-id", 0, "")
	_ = initCmd.Flags().MarkDeprecated("template-id", "use --template instead")
	_ = initCmd.Flags().MarkHidden("template-id")

	return initCmd
}

type handler struct {
	log            *zerolog.Logger
	runtimeContext *runtime.Context
	registry       RegistryInterface
	validated      bool
}

// RegistryInterface abstracts the registry for testing.
type RegistryInterface interface {
	ListTemplates(refresh bool) ([]templaterepo.TemplateSummary, error)
	GetTemplate(name string, refresh bool) (*templaterepo.TemplateSummary, error)
	ScaffoldTemplate(tmpl *templaterepo.TemplateSummary, destDir, workflowName string, onProgress func(string)) error
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:            ctx.Logger,
		runtimeContext: ctx,
		validated:      false,
	}
}

// newHandlerWithRegistry creates a handler with an injected registry (for testing).
func newHandlerWithRegistry(ctx *runtime.Context, registry RegistryInterface) *handler {
	return &handler{
		log:            ctx.Logger,
		runtimeContext: ctx,
		registry:       registry,
		validated:      false,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	templateName := v.GetString("template")

	// Handle deprecated --template-id: 1 = hello-world-go, 3 = hello-world-ts, any other = hello-world-ts
	if templateID := v.GetUint32("template-id"); templateID != 0 && templateName == "" {
		h.log.Warn().Msg("--template-id is deprecated, use --template instead")
		if templateID == 1 {
			templateName = "hello-world-go"
		} else {
			templateName = "hello-world-ts"
		}
	}

	return Inputs{
		ProjectName:  v.GetString("project-name"),
		TemplateName: templateName,
		WorkflowName: v.GetString("workflow-name"),
	}, nil
}

func (h *handler) ValidateInputs(inputs Inputs) error {
	validator, err := validation.NewValidator()
	if err != nil {
		return fmt.Errorf("failed to create validator: %w", err)
	}

	if err := validator.Struct(inputs); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	h.validated = true
	return nil
}

func (h *handler) Execute(inputs Inputs) error {
	if !h.validated {
		return fmt.Errorf("handler inputs not validated")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("unable to get working directory: %w", err)
	}
	startDir := cwd

	// Detect if we're in an existing project
	existingProjectRoot, _, existingErr := h.findExistingProject(startDir)
	isNewProject := existingErr != nil

	// Create the registry if not injected (normal flow)
	if h.registry == nil {
		sources := config.LoadTemplateSources(h.log)

		reg, err := templaterepo.NewRegistry(h.log, sources)
		if err != nil {
			return fmt.Errorf("failed to create template registry: %w", err)
		}
		h.registry = reg
	}

	refresh := h.runtimeContext.Viper.GetBool("refresh")

	// Fetch the template list
	spinner := ui.NewSpinner()
	spinner.Start("Fetching templates...")
	templates, err := h.registry.ListTemplates(refresh)
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to fetch templates: %w", err)
	}

	// Resolve template from flag if provided
	var selectedTemplate *templaterepo.TemplateSummary
	if inputs.TemplateName != "" {
		for i := range templates {
			if templates[i].Name == inputs.TemplateName {
				selectedTemplate = &templates[i]
				break
			}
		}
		if selectedTemplate == nil {
			return fmt.Errorf("template %q not found", inputs.TemplateName)
		}
	}

	// Run the interactive wizard
	result, err := RunWizard(inputs, isNewProject, templates, selectedTemplate)
	if err != nil {
		return fmt.Errorf("wizard error: %w", err)
	}
	if result.Cancelled {
		return fmt.Errorf("cre init cancelled")
	}

	// Extract values from wizard result
	projName := result.ProjectName
	workflowName := result.WorkflowName

	// Apply defaults
	if projName == "" {
		projName = constants.DefaultProjectName
	}
	if workflowName == "" {
		workflowName = constants.DefaultWorkflowName
	}

	// Resolve the selected template from wizard if not from flag
	if selectedTemplate == nil {
		selectedTemplate = result.SelectedTemplate
	}
	if selectedTemplate == nil {
		return fmt.Errorf("no template selected")
	}

	// Determine project root
	var projectRoot string
	if isNewProject {
		projectRoot = filepath.Join(startDir, projName) + "/"
	} else {
		projectRoot = existingProjectRoot
	}

	// Create project directory if new project
	if isNewProject {
		if err := h.ensureProjectDirectoryExists(projectRoot); err != nil {
			return err
		}
	}

	// Ensure env file exists for existing projects
	if !isNewProject {
		envPath := filepath.Join(projectRoot, constants.DefaultEnvFileName)
		if !h.pathExists(envPath) {
			if _, err := settings.GenerateProjectEnvFile(projectRoot); err != nil {
				return err
			}
		}
	}

	// Create project settings for new projects
	if isNewProject {
		repl := settings.GetDefaultReplacements()
		if e := settings.FindOrCreateProjectSettings(projectRoot, repl); e != nil {
			return e
		}
		if _, e := settings.GenerateProjectEnvFile(projectRoot); e != nil {
			return e
		}
	}

	// Scaffold the template
	scaffoldSpinner := ui.NewSpinner()
	scaffoldSpinner.Start("Scaffolding template...")
	err = h.registry.ScaffoldTemplate(selectedTemplate, projectRoot, workflowName, func(msg string) {
		scaffoldSpinner.Update(msg)
	})
	scaffoldSpinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to scaffold template: %w", err)
	}

	// Determine language-specific entry point
	entryPoint := "."
	if selectedTemplate.Language == "typescript" {
		entryPoint = "./main.ts"
	}

	// Generate workflow settings
	workflowDirectory := filepath.Join(projectRoot, workflowName)
	_, err = settings.GenerateWorkflowSettingsFile(workflowDirectory, workflowName, entryPoint)
	if err != nil {
		return fmt.Errorf("failed to generate %s file: %w", constants.DefaultWorkflowSettingsFileName, err)
	}

	// Show what was created
	ui.Line()
	ui.Dim("Files created in " + projectRoot)

	if h.runtimeContext != nil {
		switch selectedTemplate.Language {
		case "go":
			h.runtimeContext.Workflow.Language = constants.WorkflowLanguageGolang
		case "typescript":
			h.runtimeContext.Workflow.Language = constants.WorkflowLanguageTypeScript
		}
	}

	h.printSuccessMessage(projectRoot, workflowName, selectedTemplate.Language)

	return nil
}

// findExistingProject walks up from the given directory looking for a project settings file
func (h *handler) findExistingProject(dir string) (projectRoot string, language string, err error) {
	for {
		if h.pathExists(filepath.Join(dir, constants.DefaultProjectSettingsFileName)) {
			if h.pathExists(filepath.Join(dir, constants.DefaultIsGoFileName)) {
				return dir, "go", nil
			}
			return dir, "typescript", nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", fmt.Errorf("no existing project found")
		}
		dir = parent
	}
}

func (h *handler) printSuccessMessage(projectRoot, workflowName, language string) {
	ui.Line()
	ui.Success("Project created successfully!")
	ui.Line()

	var steps string
	if language == "go" {
		steps = ui.RenderStep("1. Navigate to your project:") + "\n" +
			"     " + ui.RenderDim("cd "+filepath.Base(projectRoot)) + "\n\n" +
			ui.RenderStep("2. Run the workflow:") + "\n" +
			"     " + ui.RenderDim("cre workflow simulate "+workflowName)
	} else {
		steps = ui.RenderStep("1. Navigate to your project:") + "\n" +
			"     " + ui.RenderDim("cd "+filepath.Base(projectRoot)) + "\n\n" +
			ui.RenderStep("2. Install Bun (if needed):") + "\n" +
			"     " + ui.RenderDim("npm install -g bun") + "\n\n" +
			ui.RenderStep("3. Install dependencies:") + "\n" +
			"     " + ui.RenderDim("bun install --cwd ./"+workflowName) + "\n\n" +
			ui.RenderStep("4. Run the workflow:") + "\n" +
			"     " + ui.RenderDim("cre workflow simulate "+workflowName)
	}

	ui.Box("Next steps\n\n" + steps)
	ui.Line()
}

func (h *handler) ensureProjectDirectoryExists(dirPath string) error {
	if h.pathExists(dirPath) {
		var overwrite bool

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Directory %s already exists. Overwrite?", dirPath)).
					Affirmative("Yes").
					Negative("No").
					Value(&overwrite),
			),
		).WithTheme(chainlinkTheme)

		if err := form.Run(); err != nil {
			return err
		}

		if !overwrite {
			return fmt.Errorf("directory creation aborted by user")
		}
		if err := os.RemoveAll(dirPath); err != nil {
			return fmt.Errorf("failed to remove existing directory %s: %w", dirPath, err)
		}
	}
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}
	return nil
}

func (h *handler) pathExists(filePath string) bool {
	_, err := os.Stat(filePath)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

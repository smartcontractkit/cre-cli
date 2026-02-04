package creinit

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

// chainlinkTheme for all Huh forms in this package
var chainlinkTheme = ui.ChainlinkTheme()

//go:embed template/workflow/**/*
var workflowTemplatesContent embed.FS

const SecretsFileName = "secrets.yaml"

type TemplateLanguage string

const (
	TemplateLangGo TemplateLanguage = "go"
	TemplateLangTS TemplateLanguage = "typescript"
)

const (
	HelloWorldTemplate string = "HelloWorld"
	PoRTemplate        string = "PoR"
	ConfHTTPTemplate   string = "ConfHTTP"
)

type WorkflowTemplate struct {
	Folder string
	Title  string
	ID     uint32
	Name   string
	Hidden bool
}

type LanguageTemplate struct {
	Title      string
	Lang       TemplateLanguage
	EntryPoint string
	Workflows  []WorkflowTemplate
}

var languageTemplates = []LanguageTemplate{
	{
		Title:      "Golang",
		Lang:       TemplateLangGo,
		EntryPoint: ".",
		Workflows: []WorkflowTemplate{
			{Folder: "porExampleDev", Title: "Custom data feed: Updating on-chain data periodically using offchain API data", ID: 1, Name: PoRTemplate},
			{Folder: "blankTemplate", Title: "Helloworld: A Golang Hello World example", ID: 2, Name: HelloWorldTemplate},
		},
	},
	{
		Title:      "Typescript",
		Lang:       TemplateLangTS,
		EntryPoint: "./main.ts",
		Workflows: []WorkflowTemplate{
			{Folder: "typescriptSimpleExample", Title: "Helloworld: Typescript Hello World example", ID: 3, Name: HelloWorldTemplate},
			{Folder: "typescriptPorExampleDev", Title: "Custom data feed: Typescript updating on-chain data periodically using offchain API data", ID: 4, Name: PoRTemplate},
			{Folder: "typescriptConfHTTP", Title: "Confidential Http: Typescript example using the confidential http capability", ID: 5, Name: ConfHTTPTemplate, Hidden: true},
		},
	},
}

type Inputs struct {
	ProjectName  string `validate:"omitempty,project_name" cli:"project-name"`
	TemplateID   uint32 `validate:"omitempty,min=0"`
	WorkflowName string `validate:"omitempty,workflow_name" cli:"workflow-name"`
	RPCUrl       string `validate:"omitempty,url" cli:"rpc-url"`
}

func New(runtimeContext *runtime.Context) *cobra.Command {
	var initCmd = &cobra.Command{
		Use:     "init",
		Aliases: []string{"new"},
		Short:   "Initialize a new cre project (recommended starting point)",
		Long: `Initialize a new CRE project or add a workflow to an existing one.

This sets up the project structure, configuration, and starter files so you can
build, test, and deploy workflows quickly.`,
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
	initCmd.Flags().Uint32P("template-id", "t", 0, "ID of the workflow template to use")
	initCmd.Flags().String("rpc-url", "", "Sepolia RPC URL to use with template")

	return initCmd
}

type handler struct {
	log            *zerolog.Logger
	clientFactory  client.Factory
	runtimeContext *runtime.Context
	validated      bool
}

func newHandler(ctx *runtime.Context) *handler {
	return &handler{
		log:            ctx.Logger,
		clientFactory:  ctx.ClientFactory,
		runtimeContext: ctx,
		validated:      false,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	return Inputs{
		ProjectName:  v.GetString("project-name"),
		TemplateID:   v.GetUint32("template-id"),
		WorkflowName: v.GetString("workflow-name"),
		RPCUrl:       v.GetString("rpc-url"),
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
	existingProjectRoot, existingProjectLanguage, existingErr := h.findExistingProject(startDir)
	isNewProject := existingErr != nil

	// If template ID provided via flag, resolve it now
	var selectedWorkflowTemplate WorkflowTemplate
	var selectedLanguageTemplate LanguageTemplate

	if inputs.TemplateID != 0 {
		wt, lt, findErr := h.getWorkflowTemplateByID(inputs.TemplateID)
		if findErr != nil {
			return fmt.Errorf("invalid template ID %d: %w", inputs.TemplateID, findErr)
		}
		selectedWorkflowTemplate = wt
		selectedLanguageTemplate = lt
	}

	// Run the interactive wizard
	result, err := RunWizard(inputs, isNewProject, existingProjectLanguage)
	if err != nil {
		return fmt.Errorf("wizard error: %w", err)
	}
	if result.Cancelled {
		return fmt.Errorf("cre init cancelled")
	}

	// Extract values from wizard result
	projName := result.ProjectName
	selectedLang := result.Language
	rpcURL := result.RPCURL
	workflowName := result.WorkflowName

	// Apply defaults
	if projName == "" {
		projName = constants.DefaultProjectName
	}
	if workflowName == "" {
		workflowName = constants.DefaultWorkflowName
	}

	// Resolve templates from wizard if not provided via flag
	if inputs.TemplateID == 0 {
		selectedLanguageTemplate, _ = h.getLanguageTemplateByTitle(selectedLang)
		selectedWorkflowTemplate, _ = h.getWorkflowTemplateByTitle(result.TemplateName, selectedLanguageTemplate.Workflows)
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
		if selectedWorkflowTemplate.Name == PoRTemplate {
			repl["EthSepoliaRpcUrl"] = rpcURL
		}
		if e := settings.FindOrCreateProjectSettings(projectRoot, repl); e != nil {
			return e
		}
		if _, e := settings.GenerateProjectEnvFile(projectRoot); e != nil {
			return e
		}
	}

	// Create workflow directory
	workflowDirectory := filepath.Join(projectRoot, workflowName)
	if err := h.ensureProjectDirectoryExists(workflowDirectory); err != nil {
		return err
	}

	projectName := filepath.Base(projectRoot)
	spinner := ui.NewSpinner()

	// Copy secrets file
	spinner.Start("Copying secrets file...")
	if err := h.copySecretsFileIfExists(projectRoot, selectedWorkflowTemplate); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to copy secrets file: %w", err)
	}

	// Generate workflow template
	spinner.Update("Generating workflow files...")
	if err := h.generateWorkflowTemplate(workflowDirectory, selectedWorkflowTemplate, projectName); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to scaffold workflow: %w", err)
	}

	// Generate contracts template
	spinner.Update("Generating contracts...")
	if err := h.generateContractsTemplate(projectRoot, selectedWorkflowTemplate, projectName); err != nil {
		spinner.Stop()
		return fmt.Errorf("failed to scaffold contracts: %w", err)
	}

	// Initialize Go module if needed
	var installedDeps *InstalledDependencies
	if selectedLanguageTemplate.Lang == TemplateLangGo {
		spinner.Update("Installing Go dependencies...")
		var goErr error
		installedDeps, goErr = initializeGoModule(h.log, projectRoot, projectName)
		if goErr != nil {
			spinner.Stop()
			return fmt.Errorf("failed to initialize Go module: %w", goErr)
		}
	}

	// Generate workflow settings
	spinner.Update("Generating workflow settings...")
	_, err = settings.GenerateWorkflowSettingsFile(workflowDirectory, workflowName, selectedLanguageTemplate.EntryPoint)
	spinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to generate %s file: %w", constants.DefaultWorkflowSettingsFileName, err)
	}

	// Show installed dependencies in a box after spinner stops
	if installedDeps != nil {
		ui.Line()
		depList := "Dependencies installed:"
		for _, dep := range installedDeps.Deps {
			depList += "\n  • " + dep
		}
		ui.Box(depList)
	}

	if h.runtimeContext != nil {
		switch selectedLanguageTemplate.Lang {
		case TemplateLangGo:
			h.runtimeContext.Workflow.Language = constants.WorkflowLanguageGolang
		case TemplateLangTS:
			h.runtimeContext.Workflow.Language = constants.WorkflowLanguageTypeScript
		}
	}

	h.printSuccessMessage(projectRoot, workflowName, selectedLanguageTemplate.Lang)

	return nil
}

// findExistingProject walks up from the given directory looking for a project settings file
func (h *handler) findExistingProject(dir string) (projectRoot string, language string, err error) {
	for {
		if h.pathExists(filepath.Join(dir, constants.DefaultProjectSettingsFileName)) {
			if h.pathExists(filepath.Join(dir, constants.DefaultIsGoFileName)) {
				return dir, "Golang", nil
			}
			return dir, "Typescript", nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", fmt.Errorf("no existing project found")
		}
		dir = parent
	}
}

func (h *handler) printSuccessMessage(projectRoot, workflowName string, lang TemplateLanguage) {
	ui.Line()
	ui.Success("Project created successfully!")
	ui.Line()

	var steps string
	if lang == TemplateLangGo {
		steps = fmt.Sprintf(`%s
     %s

%s
     %s`,
			ui.RenderStep("1. Navigate to your project:"),
			ui.RenderDim("cd "+filepath.Base(projectRoot)),
			ui.RenderStep("2. Run the workflow:"),
			ui.RenderDim("cre workflow simulate "+workflowName))
	} else {
		steps = fmt.Sprintf(`%s
     %s

%s
     %s

%s
     %s

%s
     %s`,
			ui.RenderStep("1. Navigate to your project:"),
			ui.RenderDim("cd "+filepath.Base(projectRoot)),
			ui.RenderStep("2. Install Bun (if needed):"),
			ui.RenderDim("npm install -g bun"),
			ui.RenderStep("3. Install dependencies:"),
			ui.RenderDim("bun install --cwd ./"+workflowName),
			ui.RenderStep("4. Run the workflow:"),
			ui.RenderDim("cre workflow simulate "+workflowName))
	}

	ui.Box("Next steps\n\n" + steps)
	ui.Line()
}

type TitledTemplate interface {
	GetTitle() string
}

func (w WorkflowTemplate) GetTitle() string {
	return w.Title
}

func (l LanguageTemplate) GetTitle() string {
	return l.Title
}

func (h *handler) getLanguageTemplateByTitle(title string) (LanguageTemplate, error) {
	for _, lang := range languageTemplates {
		if lang.Title == title {
			return lang, nil
		}
	}

	return LanguageTemplate{}, errors.New("language not found")
}

func (h *handler) getWorkflowTemplateByTitle(title string, workflowTemplates []WorkflowTemplate) (WorkflowTemplate, error) {
	for _, template := range workflowTemplates {
		if template.Title == title {
			return template, nil
		}
	}
	return WorkflowTemplate{}, errors.New("template not found")
}

func (h *handler) copySecretsFileIfExists(projectRoot string, template WorkflowTemplate) error {
	sourceSecretsFilePath := "template/workflow/" + template.Folder + "/" + SecretsFileName
	destinationSecretsFilePath := filepath.Join(projectRoot, SecretsFileName)

	if _, err := fs.Stat(workflowTemplatesContent, sourceSecretsFilePath); err != nil {
		h.log.Debug().Msg("Secrets file doesn't exist for this template, skipping")
		return nil
	}

	secretsFileContent, err := workflowTemplatesContent.ReadFile(sourceSecretsFilePath)
	if err != nil {
		return fmt.Errorf("failed to read secrets file: %w", err)
	}

	if err := os.WriteFile(destinationSecretsFilePath, []byte(secretsFileContent), 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	h.log.Debug().Msgf("Detected secrets file for this template, copied file to: %s", destinationSecretsFilePath)

	return nil
}

func (h *handler) generateWorkflowTemplate(workingDirectory string, template WorkflowTemplate, projectName string) error {
	h.log.Debug().Msgf("Generating template: %s", template.Title)

	templatePath := "template/workflow/" + template.Folder

	if _, err := fs.Stat(workflowTemplatesContent, templatePath); err != nil {
		return fmt.Errorf("template directory doesn't exist: %w", err)
	}

	walkErr := fs.WalkDir(workflowTemplatesContent, templatePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(templatePath, path)

		if relPath == "." {
			return nil
		}

		if strings.HasPrefix(relPath, "contracts") {
			return nil
		}

		if d.IsDir() {
			return os.MkdirAll(filepath.Join(workingDirectory, relPath), 0o755)
		}

		if strings.Contains(relPath, SecretsFileName) {
			return nil
		}

		var targetPath string
		if strings.HasSuffix(relPath, ".tpl") {
			outputFileName := strings.TrimSuffix(relPath, ".tpl")
			targetPath = filepath.Join(workingDirectory, outputFileName)
		} else {
			targetPath = filepath.Join(workingDirectory, relPath)
		}

		content, err := workflowTemplatesContent.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		finalContent := strings.ReplaceAll(string(content), "{{projectName}}", projectName)

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for: %w", err)
		}

		if err := os.WriteFile(targetPath, []byte(finalContent), 0600); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		h.log.Debug().Msgf("Copied file to: %s", targetPath)
		return nil
	})

	return walkErr
}

func (h *handler) getWorkflowTemplateByID(id uint32) (WorkflowTemplate, LanguageTemplate, error) {
	for _, lang := range languageTemplates {
		for _, tpl := range lang.Workflows {
			if tpl.ID == id {
				return tpl, lang, nil
			}
		}
	}

	return WorkflowTemplate{}, LanguageTemplate{}, fmt.Errorf("template with ID %d not found", id)
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

func (h *handler) generateContractsTemplate(projectRoot string, template WorkflowTemplate, projectName string) error {
	templateContractsPath := "template/workflow/" + template.Folder + "/contracts"

	if _, err := fs.Stat(workflowTemplatesContent, templateContractsPath); err != nil {
		return nil
	}

	h.log.Debug().Msgf("Generating contracts for template: %s", template.Title)

	contractsDirectory := filepath.Join(projectRoot, "contracts")

	walkErr := fs.WalkDir(workflowTemplatesContent, templateContractsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(templateContractsPath, path)

		if relPath == "." {
			return nil
		}

		if d.Name() == "keep.tpl" {
			return nil
		}

		if d.IsDir() {
			return os.MkdirAll(filepath.Join(contractsDirectory, relPath), 0o755)
		}

		var targetPath string
		if strings.HasSuffix(relPath, ".tpl") {
			outputFileName := strings.TrimSuffix(relPath, ".tpl")
			targetPath = filepath.Join(contractsDirectory, outputFileName)
		} else {
			targetPath = filepath.Join(contractsDirectory, relPath)
		}

		content, err := workflowTemplatesContent.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		finalContent := strings.ReplaceAll(string(content), "{{projectName}}", projectName)

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for: %w", err)
		}

		if err := os.WriteFile(targetPath, []byte(finalContent), 0600); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		h.log.Debug().Msgf("Copied contracts file to: %s", targetPath)
		return nil
	})

	return walkErr
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

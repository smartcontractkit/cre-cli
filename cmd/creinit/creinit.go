package creinit

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/smartcontractkit/cre-cli/cmd/client"
	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/prompt"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

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
)

type WorkflowTemplate struct {
	Folder string
	Title  string
	ID     uint32
	Name   string
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
		},
	},
}

type Inputs struct {
	ProjectName  string `validate:"omitempty,project_name" cli:"project-name"`
	TemplateID   uint32 `validate:"omitempty,min=0"`
	WorkflowName string `validate:"omitempty,workflow_name" cli:"workflow-name"`
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
			handler := newHandler(runtimeContext, cmd.InOrStdin())

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

	return initCmd
}

type handler struct {
	log           *zerolog.Logger
	clientFactory client.Factory
	stdin         io.Reader
	validated     bool
}

func newHandler(ctx *runtime.Context, stdin io.Reader) *handler {
	return &handler{
		log:           ctx.Logger,
		clientFactory: ctx.ClientFactory,
		stdin:         stdin,
		validated:     false,
	}
}

func (h *handler) ResolveInputs(v *viper.Viper) (Inputs, error) {
	return Inputs{
		ProjectName:  v.GetString("project-name"),
		TemplateID:   v.GetUint32("template-id"),
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

	projectRoot, existingProjectLanguage, err := func(dir string) (string, string, error) {
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
	}(startDir)

	if err != nil {
		projName := inputs.ProjectName
		if projName == "" {
			if err := prompt.SimplePrompt(h.stdin, fmt.Sprintf("Project name? [%s]", constants.DefaultProjectName), func(in string) error {
				trimmed := strings.TrimSpace(in)
				if trimmed == "" {
					trimmed = constants.DefaultProjectName
					fmt.Printf("Using default project name: %s\n", trimmed)
				}
				if err := validation.IsValidProjectName(trimmed); err != nil {
					return err
				}
				projName = filepath.Join(trimmed, "/")
				return nil
			}); err != nil {
				return err
			}
		}

		projectRoot = filepath.Join(startDir, projName)
		if err := h.ensureProjectDirectoryExists(projectRoot); err != nil {
			return err
		}
	}

	if err == nil {
		envPath := filepath.Join(projectRoot, constants.DefaultEnvFileName)
		if !h.pathExists(envPath) {
			if _, err := settings.GenerateProjectEnvFile(projectRoot, h.stdin); err != nil {
				return err
			}
		}
	}

	var selectedWorkflowTemplate WorkflowTemplate
	var selectedLanguageTemplate LanguageTemplate
	var workflowTemplates []WorkflowTemplate
	if inputs.TemplateID != 0 {
		var findErr error
		selectedWorkflowTemplate, selectedLanguageTemplate, findErr = h.getWorkflowTemplateByID(inputs.TemplateID)
		if findErr != nil {
			return fmt.Errorf("invalid template ID %d: %w", inputs.TemplateID, findErr)
		}
	} else {
		if existingProjectLanguage != "" {
			var templateErr error
			selectedLanguageTemplate, templateErr = h.getLanguageTemplateByTitle(existingProjectLanguage)
			workflowTemplates = selectedLanguageTemplate.Workflows

			if templateErr != nil {
				return fmt.Errorf("invalid template %s: %w", existingProjectLanguage, templateErr)
			}
		}

		if len(workflowTemplates) < 1 {
			languageTitles := h.extractLanguageTitles(languageTemplates)
			if err := prompt.SelectPrompt(h.stdin, "What language do you want to use?", languageTitles, func(choice string) error {
				selected, selErr := h.getLanguageTemplateByTitle(choice)
				selectedLanguageTemplate = selected
				workflowTemplates = selectedLanguageTemplate.Workflows
				return selErr
			}); err != nil {
				return fmt.Errorf("language selection aborted: %w", err)
			}
		}

		workflowTitles := h.extractWorkflowTitles(workflowTemplates)
		if err := prompt.SelectPrompt(h.stdin, "Pick a workflow template", workflowTitles, func(choice string) error {
			selected, selErr := h.getWorkflowTemplateByTitle(choice, workflowTemplates)
			selectedWorkflowTemplate = selected
			return selErr
		}); err != nil {
			return fmt.Errorf("template selection aborted: %w", err)
		}
	}

	if err != nil {
		repl := settings.GetDefaultReplacements()
		rpcURL := ""
		if selectedWorkflowTemplate.Name == PoRTemplate {
			if e := prompt.SimplePrompt(h.stdin, fmt.Sprintf("Sepolia RPC URL? [%s]", constants.DefaultEthSepoliaRpcUrl), func(in string) error {
				trimmed := strings.TrimSpace(in)
				if trimmed == "" {
					trimmed = constants.DefaultEthSepoliaRpcUrl
				}
				rpcURL = trimmed
				return nil
			}); e != nil {
				return e
			}
			repl["EthSepoliaRpcUrl"] = rpcURL
		}
		if e := settings.FindOrCreateProjectSettings(projectRoot, repl); e != nil {
			return e
		}
		if selectedWorkflowTemplate.Name == PoRTemplate {
			fmt.Printf("RPC set to %s. You can change it later in ./%s.\n",
				rpcURL,
				filepath.Join(filepath.Base(projectRoot), constants.DefaultProjectSettingsFileName))
		}
		if _, e := settings.GenerateProjectEnvFile(projectRoot, h.stdin); e != nil {
			return e
		}
	}

	workflowName := strings.TrimSpace(inputs.WorkflowName)
	if workflowName == "" {
		const maxAttempts = 3
		for attempts := 1; attempts <= maxAttempts; attempts++ {
			inputErr := prompt.SimplePrompt(h.stdin, fmt.Sprintf("Workflow name? [%s]", constants.DefaultWorkflowName), func(in string) error {
				trimmed := strings.TrimSpace(in)
				if trimmed == "" {
					trimmed = constants.DefaultWorkflowName
					fmt.Printf("Using default workflow name: %s\n", trimmed)
				}
				if err := validation.IsValidWorkflowName(trimmed); err != nil {
					return err
				}
				workflowName = trimmed
				return nil
			})

			if inputErr == nil {
				break
			}

			fmt.Fprintf(os.Stderr, "Error: %v\n", inputErr)

			if attempts == maxAttempts {
				fmt.Fprintln(os.Stderr, "Too many failed attempts. Aborting.")
				os.Exit(1)
			}
		}
	}

	workflowDirectory := filepath.Join(projectRoot, workflowName)

	if err := h.ensureProjectDirectoryExists(workflowDirectory); err != nil {
		return err
	}

	if err := h.copySecretsFileIfExists(projectRoot, selectedWorkflowTemplate); err != nil {
		return fmt.Errorf("failed to copy secrets file: %w", err)
	}

	// Get project name from project root
	projectName := filepath.Base(projectRoot)

	if err := h.generateWorkflowTemplate(workflowDirectory, selectedWorkflowTemplate, projectName); err != nil {
		return fmt.Errorf("failed to scaffold workflow: %w", err)
	}

	// Generate contracts at project level if template has contracts
	if err := h.generateContractsTemplate(projectRoot, selectedWorkflowTemplate, projectName); err != nil {
		return fmt.Errorf("failed to scaffold contracts: %w", err)
	}

	if selectedLanguageTemplate.Lang == TemplateLangGo {
		if err := initializeGoModule(h.log, projectRoot, projectName); err != nil {
			return fmt.Errorf("failed to initialize Go module: %w", err)
		}
	}

	_, err = settings.GenerateWorkflowSettingsFile(workflowDirectory, workflowName, selectedLanguageTemplate.EntryPoint)
	if err != nil {
		return fmt.Errorf("failed to generate %s file: %w", constants.DefaultWorkflowSettingsFileName, err)
	}

	fmt.Println("\nWorkflow initialized successfully!")
	fmt.Println("")
	fmt.Println("Next steps:")

	if selectedLanguageTemplate.Lang == TemplateLangGo {
		fmt.Println("   1. Navigate to your project directory:")
		fmt.Printf("      cd %s\n", filepath.Base(projectRoot))
		fmt.Println("")
		fmt.Println("   2. Run the workflow on your machine:")
		fmt.Printf("      cre workflow simulate %s\n", workflowName)
		fmt.Println("")
		fmt.Printf("   3. (Optional) Consult %s to learn more about this template:\n\n",
			filepath.Join(filepath.Base(workflowDirectory), "README.md"))
		fmt.Println("")
	} else if selectedLanguageTemplate.Lang == TemplateLangTS {
		fmt.Println("   1. Navigate to your project directory:")
		fmt.Printf("      cd %s\n", filepath.Base(projectRoot))
		fmt.Println("")
		fmt.Println("   2. Make sure you have Bun installed:")
		fmt.Println("      npm install -g bun")
		fmt.Println("")
		fmt.Println("   3. Install workflow dependencies:")
		fmt.Printf("      bun install --cwd ./%s\n", filepath.Base(workflowDirectory))
		fmt.Println("")
		fmt.Println("   4. Run the workflow on your machine:")
		fmt.Printf("      cre workflow simulate %s\n", workflowName)
		fmt.Println("")
		fmt.Printf("   5. (Optional) Consult %s to learn more about this template:\n\n",
			filepath.Join(filepath.Base(workflowDirectory), "README.md"))
		fmt.Println("")
	}
	return nil
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

func extractTitles[T TitledTemplate](templates []T) []string {
	titles := make([]string, len(templates))
	for i, template := range templates {
		titles[i] = template.GetTitle()
	}
	return titles
}

func (h *handler) extractLanguageTitles(templates []LanguageTemplate) []string {
	return extractTitles(templates)
}

func (h *handler) extractWorkflowTitles(templates []WorkflowTemplate) []string {
	return extractTitles(templates)
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

// Copy the content of the secrets file (if exists for this workflow template) to the project root
func (h *handler) copySecretsFileIfExists(projectRoot string, template WorkflowTemplate) error {
	// When referencing embedded template files, the path is relative and separated by forward slashes
	sourceSecretsFilePath := "template/workflow/" + template.Folder + "/" + SecretsFileName
	destinationSecretsFilePath := filepath.Join(projectRoot, SecretsFileName)

	// Ensure the secrets file exists in the template directory
	if _, err := fs.Stat(workflowTemplatesContent, sourceSecretsFilePath); err != nil {
		fmt.Println("Secrets file doesn't exist for this template, skipping")
		return nil
	}

	// Read the content of the secrets file from the template
	secretsFileContent, err := workflowTemplatesContent.ReadFile(sourceSecretsFilePath)
	if err != nil {
		return fmt.Errorf("failed to read secrets file: %w", err)
	}

	// Write the file content to the target path
	if err := os.WriteFile(destinationSecretsFilePath, []byte(secretsFileContent), 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	h.log.Debug().Msgf("Detected secrets file for this template, copied file to: %s", destinationSecretsFilePath)

	return nil
}

// Copy the content of template/workflow/{{templateName}} and remove "tpl" extension
func (h *handler) generateWorkflowTemplate(workingDirectory string, template WorkflowTemplate, projectName string) error {

	fmt.Printf("Generating template: %s\n", template.Title)

	// Construct the path to the specific template directory
	// When referencing embedded template files, the path is relative and separated by forward slashes
	templatePath := "template/workflow/" + template.Folder

	// Ensure the specified template directory exists
	if _, err := fs.Stat(workflowTemplatesContent, templatePath); err != nil {
		return fmt.Errorf("template directory doesn't exist: %w", err)
	}

	// Walk through all files & folders under templatePath
	walkErr := fs.WalkDir(workflowTemplatesContent, templatePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // propagate I/O errors
		}

		// Compute the path of this entry relative to templatePath
		relPath, _ := filepath.Rel(templatePath, path)

		// Skip the top-level directory itself
		if relPath == "." {
			return nil
		}

		// Skip contracts directory - it will be handled separately
		if strings.HasPrefix(relPath, "contracts") {
			return nil
		}

		// If it's a directory, just create the matching directory in the working dir
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(workingDirectory, relPath), 0o755)
		}

		// Skip the secrets file if it exists, this one is copied separately into the project root
		if strings.Contains(relPath, SecretsFileName) {
			return nil
		}

		// Determine the target file path
		var targetPath string
		if strings.HasSuffix(relPath, ".tpl") {
			// Remove `.tpl` extension for files with `.tpl`
			outputFileName := strings.TrimSuffix(relPath, ".tpl")
			targetPath = filepath.Join(workingDirectory, outputFileName)
		} else {
			// Copy other files as-is
			targetPath = filepath.Join(workingDirectory, relPath)
		}

		// Read the file content
		content, err := workflowTemplatesContent.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Replace template variables with actual values
		finalContent := strings.ReplaceAll(string(content), "{{projectName}}", projectName)

		// Ensure the target directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for: %w", err)
		}

		// Write the file content to the target path
		if err := os.WriteFile(targetPath, []byte(finalContent), 0600); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		h.log.Debug().Msgf("Copied file to: %s", targetPath)
		return nil
	})

	fmt.Printf("Files created in %s directory\n", workingDirectory)

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
		overwrite, err := prompt.YesNoPrompt(
			h.stdin,
			fmt.Sprintf("Directory %s already exists. Overwrite?", dirPath),
		)
		if err != nil {
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
	// Construct the path to the contracts directory in the template
	// When referencing embedded template files, the path is relative and separated by forward slashes
	templateContractsPath := "template/workflow/" + template.Folder + "/contracts"

	// Check if this template has contracts
	if _, err := fs.Stat(workflowTemplatesContent, templateContractsPath); err != nil {
		// No contracts directory in this template, skip
		return nil
	}

	h.log.Debug().Msgf("Generating contracts for template: %s", template.Title)

	// Create contracts directory at project level
	contractsDirectory := filepath.Join(projectRoot, "contracts")

	// Walk through all files & folders under contracts template
	walkErr := fs.WalkDir(workflowTemplatesContent, templateContractsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // propagate I/O errors
		}

		// Compute the path of this entry relative to templateContractsPath
		relPath, _ := filepath.Rel(templateContractsPath, path)

		// Skip the top-level directory itself
		if relPath == "." {
			return nil
		}

		// Skip keep.tpl file used to copy empty directory
		if d.Name() == "keep.tpl" {
			return nil
		}

		// If it's a directory, just create the matching directory in the contracts dir
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(contractsDirectory, relPath), 0o755)
		}

		// Determine the target file path
		var targetPath string
		if strings.HasSuffix(relPath, ".tpl") {
			// Remove `.tpl` extension for files with `.tpl`
			outputFileName := strings.TrimSuffix(relPath, ".tpl")
			targetPath = filepath.Join(contractsDirectory, outputFileName)
		} else {
			// Copy other files as-is
			targetPath = filepath.Join(contractsDirectory, relPath)
		}

		// Read the file content
		content, err := workflowTemplatesContent.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		// Replace template variables with actual values
		finalContent := strings.ReplaceAll(string(content), "{{projectName}}", projectName)

		// Ensure the target directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for: %w", err)
		}

		// Write the file content to the target path
		if err := os.WriteFile(targetPath, []byte(finalContent), 0600); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		h.log.Debug().Msgf("Copied contracts file to: %s", targetPath)
		return nil
	})

	fmt.Printf("Contracts generated under %s\n", templateContractsPath)

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

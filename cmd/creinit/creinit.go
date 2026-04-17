package creinit

import (
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/templateconfig"
	"github.com/smartcontractkit/cre-cli/internal/templaterepo"
	"github.com/smartcontractkit/cre-cli/internal/tenderly"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

type Inputs struct {
	ProjectName    string            `validate:"omitempty,project_name" cli:"project-name"`
	TemplateName   string            `validate:"omitempty" cli:"template"`
	WorkflowName   string            `validate:"omitempty,workflow_name" cli:"workflow-name"`
	RpcURLs        map[string]string // chain-name -> url, from --rpc-url flags
	UseTenderly    bool              // use Tenderly Virtual Networks for RPCs
	NonInteractive bool
	ProjectRoot    string // from -R / --project-root flag
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
			h := newHandler(runtimeContext)

			inputs, err := h.ResolveInputs(runtimeContext.Viper)
			if err != nil {
				return err
			}

			// Only use -R if the user explicitly passed it on the command line
			if cmd.Flags().Changed(settings.Flags.ProjectRoot.Name) {
				inputs.ProjectRoot = runtimeContext.Viper.GetString(settings.Flags.ProjectRoot.Name)
			}
			if err = h.ValidateInputs(inputs); err != nil {
				return err
			}
			execErr := h.Execute(inputs)

			// Ensure --template is marked as set for telemetry, even when
			// the user picked a template interactively via the wizard.
			if h.selectedTemplateName != "" {
				_ = cmd.Flags().Set("template", h.selectedTemplateName)
			}

			return execErr
		},
	}

	initCmd.Flags().StringP("project-name", "p", "", "Name for the new project")
	initCmd.Flags().StringP("workflow-name", "w", "", "Name for the new workflow")
	initCmd.Flags().StringP("template", "t", "", "Name of the template to use (e.g., kv-store-go)")
	initCmd.Flags().Bool("refresh", false, "Bypass template cache and fetch fresh data")
	initCmd.Flags().StringArray("rpc-url", nil, "RPC URL for a network (format: chain-name=url, repeatable)")
	initCmd.Flags().Bool("use-tenderly", false, "Use Tenderly Virtual Networks for RPC URLs")
	initCmd.Flags().Bool("non-interactive", false, "Fail instead of prompting; requires all inputs via flags")

	// Deprecated: --template-id is kept for backwards compatibility, maps to hello-world-go
	initCmd.Flags().Uint32("template-id", 0, "")
	_ = initCmd.Flags().MarkDeprecated("template-id", "use --template instead")
	_ = initCmd.Flags().MarkHidden("template-id")

	return initCmd
}

type handler struct {
	log                  *zerolog.Logger
	runtimeContext       *runtime.Context
	registry             RegistryInterface
	tenderlyProvider     tenderly.Provider // nil = create from env at runtime
	validated            bool
	selectedTemplateName string // set after Execute for telemetry
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

	// Handle deprecated --template-id: 1,2 = hello-world-go, 3+ = hello-world-ts
	if templateID := v.GetUint32("template-id"); templateID != 0 && templateName == "" {
		h.log.Warn().Msg("--template-id is deprecated, use --template instead")
		if templateID <= 2 {
			templateName = "hello-world-go"
		} else {
			templateName = "hello-world-ts"
		}
	}

	// Parse --rpc-url flag values (chain-name=url)
	rpcURLs := make(map[string]string)
	for _, raw := range v.GetStringSlice("rpc-url") {
		parts := strings.SplitN(raw, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return Inputs{}, fmt.Errorf("invalid --rpc-url format %q: expected chain-name=url", raw)
		}
		rpcURLs[parts[0]] = parts[1]
	}

	return Inputs{
		ProjectName:    v.GetString("project-name"),
		TemplateName:   templateName,
		WorkflowName:   v.GetString("workflow-name"),
		RpcURLs:        rpcURLs,
		UseTenderly:    v.GetBool("use-tenderly"),
		NonInteractive: v.GetBool("non-interactive"),
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

	// Ensure the default template config exists on first run
	if err := templateconfig.EnsureDefaultConfig(h.log); err != nil {
		h.log.Warn().Err(err).Msg("Failed to create default template config")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("unable to get working directory: %w", err)
	}
	startDir := cwd

	// Respect -R / --project-root flag if provided.
	// For init, treat -R as the base directory for project creation.
	// The directory does not need to exist yet — it will be created during scaffolding.
	if inputs.ProjectRoot != "" {
		absRoot, err := filepath.Abs(inputs.ProjectRoot)
		if err != nil {
			return fmt.Errorf("invalid --project-root path: %w", err)
		}
		// If -R points to a file, that's a user error — it must be a directory
		if info, err := os.Stat(absRoot); err == nil && !info.IsDir() {
			return fmt.Errorf("--project-root %q is a file, not a directory", inputs.ProjectRoot)
		}
		startDir = absRoot
	}

	// Detect if we're in an existing project
	existingProjectRoot, _, existingErr := h.findExistingProject(startDir)
	isNewProject := existingErr != nil

	// Create the registry if not injected (normal flow)
	if h.registry == nil {
		sources := templateconfig.LoadTemplateSources(h.log)

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

	// Filter to only workflow templates (category == "workflow")
	var workflowTemplates []templaterepo.TemplateSummary
	for _, t := range templates {
		if t.Category == templaterepo.CategoryWorkflow {
			workflowTemplates = append(workflowTemplates, t)
		}
	}

	// Resolve template from flag if provided
	var selectedTemplate *templaterepo.TemplateSummary
	if inputs.TemplateName != "" {
		for i := range workflowTemplates {
			if workflowTemplates[i].Name == inputs.TemplateName || workflowTemplates[i].ID == inputs.TemplateName {
				selectedTemplate = &workflowTemplates[i]
				break
			}
		}
		if selectedTemplate == nil {
			return fmt.Errorf("template %q not found. Run 'cre templates list' to see all available templates", inputs.TemplateName)
		}
	}

	// Non-interactive mode: validate all required inputs are present
	if inputs.NonInteractive {
		var missingFlags []string
		if isNewProject && inputs.ProjectName == "" {
			missingFlags = append(missingFlags, "--project-name")
		}
		if inputs.TemplateName == "" {
			missingFlags = append(missingFlags, "--template")
		}
		if selectedTemplate != nil {
			missing := MissingNetworks(selectedTemplate, inputs.RpcURLs)
			for _, network := range missing {
				missingFlags = append(missingFlags, fmt.Sprintf("--rpc-url=\"%s=<url>\"", network))
			}
			if inputs.WorkflowName == "" && selectedTemplate.ProjectDir == "" && len(selectedTemplate.Workflows) <= 1 {
				missingFlags = append(missingFlags, "--workflow-name")
			}
		}
		if len(missingFlags) > 0 {
			ui.ErrorWithSuggestions(
				"Non-interactive mode requires all inputs via flags",
				missingFlags,
			)
			return fmt.Errorf("missing required flags for --non-interactive mode")
		}
	}

	// Run the interactive wizard
	result, err := RunWizard(inputs, isNewProject, startDir, workflowTemplates, selectedTemplate)
	if err != nil {
		// If stdin is not a terminal, the wizard will fail trying to open a TTY.
		// Detect this via term.IsTerminal rather than matching third-party error strings.
		if !term.IsTerminal(int(os.Stdin.Fd())) { // #nosec G115 -- stdin fd is always 0
			var suggestions []string
			if selectedTemplate != nil {
				missing := MissingNetworks(selectedTemplate, inputs.RpcURLs)
				for _, network := range missing {
					suggestions = append(suggestions, fmt.Sprintf("--rpc-url=\"%s=<url>\"", network))
				}
			}
			if len(suggestions) > 0 {
				ui.ErrorWithSuggestions(
					"Interactive mode requires a terminal (TTY). Provide the missing flags to run non-interactively",
					suggestions,
				)
			} else {
				ui.Error("Interactive mode requires a terminal (TTY). Use --non-interactive with all required flags, or run in a terminal")
			}
			return fmt.Errorf("interactive mode requires a terminal (TTY)")
		}
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
		if selectedTemplate != nil && len(selectedTemplate.Workflows) == 1 {
			workflowName = selectedTemplate.Workflows[0].Dir
		} else {
			workflowName = constants.DefaultWorkflowName
		}
	}

	// Resolve the selected template from wizard if not from flag
	if selectedTemplate == nil {
		selectedTemplate = result.SelectedTemplate
	}
	if selectedTemplate == nil {
		return fmt.Errorf("no template selected")
	}

	// Store for telemetry (flag will be set in RunE)
	h.selectedTemplateName = selectedTemplate.Name

	// Determine project root
	var projectRoot string
	if isNewProject {
		projectRoot = filepath.Join(startDir, projName) + "/"
	} else {
		projectRoot = existingProjectRoot
	}

	// Create project directory if new project
	if isNewProject {
		if err := h.ensureProjectDirectoryExists(projectRoot, result.OverwriteDir); err != nil {
			return err
		}
	}

	// Tenderly Virtual Networks: provision RPC URLs if requested
	var vnetResult *tenderly.VnetResult
	if result.UseTenderly || inputs.UseTenderly {
		if len(selectedTemplate.Networks) > 0 {
			provider := h.tenderlyProvider
			if provider == nil {
				creds, credErr := credentials.New(h.log)
				if credErr != nil {
					return fmt.Errorf("failed to load credentials for Tenderly (run cre login first): %w", credErr)
				}
				userID, uidErr := creds.GetUserID()
				if uidErr != nil {
					return fmt.Errorf("failed to get user ID for Tenderly: %w", uidErr)
				}
				var providerErr error
				provider, providerErr = tenderly.NewAPIProvider(userID)
				if providerErr != nil {
					return fmt.Errorf("failed to initialize Tenderly provider: %w", providerErr)
				}
			}
			var vnetErr error
			vnetResult, vnetErr = provider.CreateVnets(selectedTemplate.Networks)
			if vnetErr != nil {
				return fmt.Errorf("failed to create Tenderly vnets: %w", vnetErr)
			}
		}
	}

	// Merge RPC URLs from wizard + flags (flags take precedence)
	networkRPCs := result.NetworkRPCs
	if networkRPCs == nil {
		networkRPCs = make(map[string]string)
	}
	// Apply Tenderly vnet URLs as base (before wizard and flag RPCs)
	if vnetResult != nil {
		for chain, rpcURL := range vnetResult.NetworkRPCs {
			networkRPCs[chain] = rpcURL
		}
	}
	maps.Copy(networkRPCs, inputs.RpcURLs)
	// Validate any provided RPC URLs
	for chain, rpcURL := range networkRPCs {
		if rpcURL != "" {
			if u, parseErr := url.Parse(rpcURL); parseErr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return fmt.Errorf("invalid RPC URL for %s: must be a valid http/https URL", chain)
			}
		}
	}

	// Scaffold the template first — remote templates include project.yaml, .env, etc.
	scaffoldSpinner := ui.NewSpinner()
	scaffoldSpinner.Start("Scaffolding template...")
	err = h.registry.ScaffoldTemplate(selectedTemplate, projectRoot, workflowName, func(msg string) {
		scaffoldSpinner.Update(msg)
	})
	scaffoldSpinner.Stop()
	if err != nil {
		return fmt.Errorf("failed to scaffold template: %w", err)
	}

	// Patch RPC URLs into project.yaml for all templates (including those with projectDir).
	// Templates that ship their own project.yaml still need user-provided RPCs applied.
	projectYAMLPath := filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName)
	if isNewProject && h.pathExists(projectYAMLPath) {
		if err := settings.PatchProjectRPCs(projectYAMLPath, networkRPCs); err != nil {
			return fmt.Errorf("failed to update RPC URLs in project.yaml: %w", err)
		}
	}

	// Templates with projectDir provide their own project structure — skip config generation.
	// Only built-in templates (no projectDir) need config files generated by the CLI.
	if selectedTemplate.ProjectDir == "" {
		// Generate project.yaml if the template didn't provide one
		if isNewProject && !h.pathExists(projectYAMLPath) {
			networks := selectedTemplate.Networks
			repl := settings.GetReplacementsWithNetworks(networks, networkRPCs)
			if e := settings.FindOrCreateProjectSettings(projectRoot, repl); e != nil {
				return e
			}
		}

		// Initialize Go module if needed
		if selectedTemplate.Language == constants.WorkflowLanguageGolang && !h.pathExists(filepath.Join(projectRoot, "go.mod")) {
			projectName := filepath.Base(projectRoot)
			if _, err := initializeGoModule(h.log, projectRoot, projectName); err != nil {
				return fmt.Errorf("failed to initialize Go module: %w", err)
			}
		}

		// Generate workflow settings
		entryPoint := "."
		if selectedTemplate.Language == constants.WorkflowLanguageTypeScript {
			entryPoint = "./main.ts"
		}

		if len(selectedTemplate.Workflows) > 1 {
			for _, wf := range selectedTemplate.Workflows {
				wfDir := filepath.Join(projectRoot, wf.Dir)
				wfSettingsPath := filepath.Join(wfDir, constants.DefaultWorkflowSettingsFileName)
				if _, err := os.Stat(wfSettingsPath); err == nil {
					h.log.Debug().Msgf("Skipping workflow.yaml generation for %s (already exists from template)", wf.Dir)
					continue
				}
				if _, err := settings.GenerateWorkflowSettingsFile(wfDir, wf.Dir, entryPoint); err != nil {
					return fmt.Errorf("failed to generate workflow settings for %s: %w", wf.Dir, err)
				}
			}
		} else {
			workflowDirectory := filepath.Join(projectRoot, workflowName)
			wfSettingsPath := filepath.Join(workflowDirectory, constants.DefaultWorkflowSettingsFileName)
			if _, err := os.Stat(wfSettingsPath); err == nil {
				h.log.Debug().Msgf("Skipping workflow.yaml generation (already exists from template)")
			} else if _, err := settings.GenerateWorkflowSettingsFile(workflowDirectory, workflowName, entryPoint); err != nil {
				return fmt.Errorf("failed to generate %s file: %w", constants.DefaultWorkflowSettingsFileName, err)
			}
		}
	}

	// For templates with projectDir, patch project.yaml with user-provided or vnet RPC URLs.
	// The config generation block above only handles templates without projectDir.
	if selectedTemplate.ProjectDir != "" && len(networkRPCs) > 0 {
		projectYAMLPath := filepath.Join(projectRoot, constants.DefaultProjectSettingsFileName)
		if h.pathExists(projectYAMLPath) {
			if err := settings.PatchProjectRPCs(projectYAMLPath, networkRPCs); err != nil {
				return fmt.Errorf("failed to update RPC URLs in project.yaml: %w", err)
			}
		}
	}

	// Ensure .env exists — dynamic templates with projectDir may not ship one
	envPath := filepath.Join(projectRoot, constants.DefaultEnvFileName)
	if !h.pathExists(envPath) {
		if _, e := settings.GenerateProjectEnvFile(projectRoot); e != nil {
			return e
		}
	}

	// For templates that ship their own go.mod (projectDir set), run go mod tidy
	// to ensure go.sum is populated after extraction.
	if selectedTemplate.Language == constants.WorkflowLanguageGolang && h.pathExists(filepath.Join(projectRoot, "go.mod")) {
		if err := runCommand(h.log, projectRoot, "go", "mod", "tidy"); err != nil {
			h.log.Warn().Err(err).Msg("go mod tidy failed; you may need to run it manually")
		}
	}

	// Install contracts dependencies for TypeScript projects when a contracts/package.json exists
	if selectedTemplate.Language == constants.WorkflowLanguageTypeScript {
		contractsPkg := filepath.Join(projectRoot, "contracts", "package.json")
		if h.pathExists(contractsPkg) {
			spinner.Update("Installing contracts dependencies...")
			if err := runBunInstall(h.log, filepath.Join(projectRoot, "contracts")); err != nil {
				spinner.Stop()
				return fmt.Errorf("failed to install contracts dependencies: %w", err)
			}
		}
	}

	// Show what was created
	ui.Line()
	ui.Dim("Files created in " + projectRoot)

	if h.runtimeContext != nil {
		h.runtimeContext.Workflow.Language = selectedTemplate.Language
	}

	h.printSuccessMessage(projectRoot, selectedTemplate, workflowName, vnetResult)

	return nil
}

// findExistingProject walks up from the given directory looking for a project settings file
func (h *handler) findExistingProject(dir string) (projectRoot string, language string, err error) {
	for {
		if h.pathExists(filepath.Join(dir, constants.DefaultProjectSettingsFileName)) {
			if h.pathExists(filepath.Join(dir, constants.DefaultIsGoFileName)) {
				return dir, constants.WorkflowLanguageGolang, nil
			}
			return dir, constants.WorkflowLanguageTypeScript, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", fmt.Errorf("no existing project found")
		}
		dir = parent
	}
}

func (h *handler) printSuccessMessage(projectRoot string, tmpl *templaterepo.TemplateSummary, workflowName string, vnetResult *tenderly.VnetResult) {
	language := tmpl.Language
	workflows := tmpl.Workflows
	isMultiWorkflow := len(workflows) > 1

	ui.Line()
	ui.Success("Project created successfully!")
	ui.Line()

	// Workflow summary (multi-workflow only, shown BEFORE the box)
	if isMultiWorkflow {
		fmt.Printf("  This template includes %d workflows:\n", len(workflows))
		for _, wf := range workflows {
			if wf.Description != "" {
				fmt.Printf("    - %s — %s\n", wf.Dir, wf.Description)
			} else {
				fmt.Printf("    - %s\n", wf.Dir)
			}
		}
		ui.Line()
	}

	// Determine which workflow name to use in example commands
	primaryWorkflow := workflowName
	if isMultiWorkflow {
		primaryWorkflow = workflows[0].Dir
	}

	var sb strings.Builder
	if language == constants.WorkflowLanguageGolang {
		sb.WriteString(ui.RenderStep("1. Navigate to your project:") + "\n")
		sb.WriteString("     " + ui.RenderDim("cd "+filepath.Base(projectRoot)) + "\n\n")

		if isMultiWorkflow {
			sb.WriteString(ui.RenderStep("2. Run a workflow:") + "\n")
			for _, wf := range workflows {
				sb.WriteString("     " + ui.RenderDim("cre workflow simulate "+wf.Dir) + "\n")
			}
		} else {
			sb.WriteString(ui.RenderStep("2. Run the workflow:") + "\n")
			sb.WriteString("     " + ui.RenderDim("cre workflow simulate "+primaryWorkflow))
		}
	} else {
		sb.WriteString(ui.RenderStep("1. Navigate to your project:") + "\n")
		sb.WriteString("     " + ui.RenderDim("cd "+filepath.Base(projectRoot)) + "\n\n")
		sb.WriteString(ui.RenderStep("2. Install Bun (if needed):") + "\n")
		sb.WriteString("     " + ui.RenderDim("npm install -g bun") + "\n\n")
		sb.WriteString(ui.RenderStep("3. Install dependencies:") + "\n")
		if isMultiWorkflow {
			for _, wf := range workflows {
				sb.WriteString("     " + ui.RenderDim("bun install --cwd ./"+wf.Dir) + "\n")
			}
		} else {
			sb.WriteString("     " + ui.RenderDim("bun install --cwd ./"+primaryWorkflow) + "\n")
		}
		sb.WriteString("\n")

		if isMultiWorkflow {
			sb.WriteString(ui.RenderStep("4. Run a workflow:") + "\n")
			for _, wf := range workflows {
				sb.WriteString("     " + ui.RenderDim("cre workflow simulate "+wf.Dir) + "\n")
			}
		} else {
			sb.WriteString(ui.RenderStep("4. Run the workflow:") + "\n")
			sb.WriteString("     " + ui.RenderDim("cre workflow simulate "+primaryWorkflow))
		}
	}

	steps := sb.String()

	ui.Box("Next steps\n\n" + steps)
	ui.Line()

	// postInit: template-specific prerequisites (OUTSIDE the box)
	if tmpl.PostInit != "" {
		fmt.Println("  " + strings.TrimSpace(tmpl.PostInit))
		ui.Line()
	}

	// Tenderly Virtual Networks info
	if vnetResult != nil && len(vnetResult.VnetURLs) > 0 {
		fmt.Println("  Tenderly Virtual TestNets")
		for chain, vnetURL := range vnetResult.VnetURLs {
			fmt.Printf("    %s:\n", chain)
			fmt.Printf("      Dashboard: %s\n", vnetURL)
			if publicRPC, ok := vnetResult.PublicRPCs[chain]; ok && publicRPC != "" {
				fmt.Printf("      Public RPC: %s\n", publicRPC)
			}
		}
		fmt.Println("  Public RPCs can be shared with anyone — no Tenderly account needed.")
		fmt.Println("  Use the dashboard to inspect transactions and debug chain readers/writers.")
		ui.Line()
	}
}

func (h *handler) ensureProjectDirectoryExists(dirPath string, alreadyConfirmedOverwrite bool) error {
	if h.pathExists(dirPath) {
		if alreadyConfirmedOverwrite {
			// User already confirmed overwrite in the wizard
			if err := os.RemoveAll(dirPath); err != nil {
				return fmt.Errorf("failed to remove existing directory %s: %w", dirPath, err)
			}
		} else {
			overwrite, err := ui.Confirm(
				fmt.Sprintf("Directory %s already exists. Overwrite?", dirPath),
				ui.WithLabels("Yes", "No"),
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

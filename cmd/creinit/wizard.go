package creinit

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

// Wizard steps
type wizardStep int

const (
	stepProjectName wizardStep = iota
	stepLanguage
	stepTemplate
	stepRPCUrl
	stepWorkflowName
	stepDone
)

// wizardModel is the Bubble Tea model for the init wizard
type wizardModel struct {
	// Current step
	step wizardStep

	// Form values
	projectName  string
	language     string
	templateName string
	rpcURL       string
	workflowName string

	// Text inputs
	projectInput  textinput.Model
	rpcInput      textinput.Model
	workflowInput textinput.Model

	// Select state
	languageOptions []string
	languageCursor  int
	templateOptions []string
	templateTitles  []string // Full titles for lookup
	templateCursor  int

	// Flags to skip steps
	skipProjectName  bool
	skipLanguage     bool
	skipTemplate     bool
	skipRPCUrl       bool
	skipWorkflowName bool

	// Whether PoR template is selected (needs RPC URL)
	needsRPC bool

	// Error message for validation
	err string

	// Whether wizard completed successfully
	completed bool
	cancelled bool

	// Styles
	titleStyle    lipgloss.Style
	dimStyle      lipgloss.Style
	promptStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	cursorStyle   lipgloss.Style
	helpStyle     lipgloss.Style
}

// WizardResult contains the wizard output
type WizardResult struct {
	ProjectName  string
	Language     string
	TemplateName string
	RPCURL       string
	WorkflowName string
	Completed    bool
	Cancelled    bool
}

// newWizardModel creates a new wizard model
func newWizardModel(inputs Inputs, isNewProject bool, existingLanguage string) wizardModel {
	// Project name input
	pi := textinput.New()
	pi.Placeholder = constants.DefaultProjectName
	pi.CharLimit = 64
	pi.Width = 40

	// RPC URL input
	ri := textinput.New()
	ri.Placeholder = constants.DefaultEthSepoliaRpcUrl
	ri.CharLimit = 256
	ri.Width = 60

	// Workflow name input
	wi := textinput.New()
	wi.Placeholder = constants.DefaultWorkflowName
	wi.CharLimit = 64
	wi.Width = 40

	// Language options
	langOpts := make([]string, len(languageTemplates))
	for i, lang := range languageTemplates {
		langOpts[i] = lang.Title
	}

	m := wizardModel{
		step:            stepProjectName,
		projectInput:    pi,
		rpcInput:        ri,
		workflowInput:   wi,
		languageOptions: langOpts,

		// Styles using ui package colors
		titleStyle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ui.ColorBlue500)),
		dimStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray500)),
		promptStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ui.ColorBlue400)),
		selectedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue500)),
		cursorStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue500)),
		helpStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray500)),
	}

	// Handle pre-populated values and skip flags
	if !isNewProject {
		m.skipProjectName = true
		m.language = existingLanguage
		m.skipLanguage = true
	}

	if inputs.ProjectName != "" {
		m.projectName = inputs.ProjectName
		m.skipProjectName = true
	}

	if inputs.TemplateID != 0 {
		m.skipLanguage = true
		m.skipTemplate = true
		// Will be resolved by handler
	}

	if inputs.RPCUrl != "" {
		m.rpcURL = inputs.RPCUrl
		m.skipRPCUrl = true
	}

	if inputs.WorkflowName != "" {
		m.workflowName = inputs.WorkflowName
		m.skipWorkflowName = true
	}

	// Start at the right step
	m.advanceToNextStep()

	return m
}

func (m *wizardModel) advanceToNextStep() {
	for {
		switch m.step {
		case stepProjectName:
			if m.skipProjectName {
				m.step++
				continue
			}
			m.projectInput.Focus()
			return
		case stepLanguage:
			if m.skipLanguage {
				m.step++
				m.updateTemplateOptions()
				continue
			}
			return
		case stepTemplate:
			if m.skipTemplate {
				m.step++
				continue
			}
			m.updateTemplateOptions()
			return
		case stepRPCUrl:
			// Check if we need RPC URL
			if m.skipRPCUrl || !m.needsRPC {
				m.step++
				continue
			}
			m.rpcInput.Focus()
			return
		case stepWorkflowName:
			if m.skipWorkflowName {
				m.step++
				continue
			}
			m.workflowInput.Focus()
			return
		case stepDone:
			m.completed = true
			return
		}
	}
}

func (m *wizardModel) updateTemplateOptions() {
	lang := m.language
	if lang == "" && m.languageCursor < len(m.languageOptions) {
		lang = m.languageOptions[m.languageCursor]
	}

	for _, lt := range languageTemplates {
		if lt.Title == lang {
			m.templateOptions = nil
			m.templateTitles = nil
			for _, wt := range lt.Workflows {
				if !wt.Hidden {
					// Use short label for display
					parts := strings.SplitN(wt.Title, ": ", 2)
					label := wt.Title
					if len(parts) == 2 {
						label = parts[0]
					}
					m.templateOptions = append(m.templateOptions, label)
					m.templateTitles = append(m.templateTitles, wt.Title)
				}
			}
			break
		}
	}
	m.templateCursor = 0
}

func (m wizardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear error on any key
		m.err = ""

		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.step == stepLanguage && m.languageCursor > 0 {
				m.languageCursor--
			} else if m.step == stepTemplate && m.templateCursor > 0 {
				m.templateCursor--
			}

		case "down", "j":
			if m.step == stepLanguage && m.languageCursor < len(m.languageOptions)-1 {
				m.languageCursor++
			} else if m.step == stepTemplate && m.templateCursor < len(m.templateOptions)-1 {
				m.templateCursor++
			}
		}
	}

	// Update text inputs
	var cmd tea.Cmd
	switch m.step {
	case stepProjectName:
		m.projectInput, cmd = m.projectInput.Update(msg)
	case stepRPCUrl:
		m.rpcInput, cmd = m.rpcInput.Update(msg)
	case stepWorkflowName:
		m.workflowInput, cmd = m.workflowInput.Update(msg)
	case stepLanguage, stepTemplate, stepDone:
		// No text input to update for these steps
	}

	return m, cmd
}

func (m wizardModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepProjectName:
		value := m.projectInput.Value()
		if value == "" {
			value = constants.DefaultProjectName
		}
		if err := validation.IsValidProjectName(value); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.projectName = value
		m.step++
		m.advanceToNextStep()

	case stepLanguage:
		m.language = m.languageOptions[m.languageCursor]
		m.step++
		m.advanceToNextStep()

	case stepTemplate:
		m.templateName = m.templateTitles[m.templateCursor]
		// Check if this is PoR template
		for _, lt := range languageTemplates {
			if lt.Title == m.language {
				for _, wt := range lt.Workflows {
					if wt.Title == m.templateName {
						m.needsRPC = (wt.Name == PoRTemplate)
						break
					}
				}
				break
			}
		}
		m.step++
		m.advanceToNextStep()

	case stepRPCUrl:
		value := m.rpcInput.Value()
		if value == "" {
			value = constants.DefaultEthSepoliaRpcUrl
		}
		m.rpcURL = value
		m.step++
		m.advanceToNextStep()

	case stepWorkflowName:
		value := m.workflowInput.Value()
		if value == "" {
			value = constants.DefaultWorkflowName
		}
		if err := validation.IsValidWorkflowName(value); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.workflowName = value
		m.step++
		m.advanceToNextStep()

	case stepDone:
		// Already done, nothing to do
	}

	if m.completed {
		return m, tea.Quit
	}

	return m, nil
}

func (m wizardModel) View() string {
	if m.cancelled {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(m.titleStyle.Render("Create a new CRE project"))
	b.WriteString("\n\n")

	// History of completed steps
	if m.projectName != "" && m.step > stepProjectName {
		b.WriteString(m.dimStyle.Render("  Project: " + m.projectName))
		b.WriteString("\n")
	}
	if m.language != "" && m.step > stepLanguage {
		b.WriteString(m.dimStyle.Render("  Language: " + m.language))
		b.WriteString("\n")
	}
	if m.templateName != "" && m.step > stepTemplate {
		label := m.templateName
		parts := strings.SplitN(label, ": ", 2)
		if len(parts) == 2 {
			label = parts[0]
		}
		b.WriteString(m.dimStyle.Render("  Template: " + label))
		b.WriteString("\n")
	}
	if m.rpcURL != "" && m.step > stepRPCUrl && m.needsRPC {
		b.WriteString(m.dimStyle.Render("  RPC URL: " + m.rpcURL))
		b.WriteString("\n")
	}

	// Add spacing before current prompt if we have history
	if m.step > stepProjectName && !m.skipProjectName {
		b.WriteString("\n")
	}

	// Current step prompt
	switch m.step {
	case stepProjectName:
		b.WriteString(m.promptStyle.Render("  Project name"))
		b.WriteString("\n")
		b.WriteString(m.dimStyle.Render("  Name for your new CRE project"))
		b.WriteString("\n\n")
		b.WriteString("  ")
		b.WriteString(m.projectInput.View())
		b.WriteString("\n")

	case stepLanguage:
		b.WriteString(m.promptStyle.Render("  What language do you want to use?"))
		b.WriteString("\n\n")
		for i, opt := range m.languageOptions {
			cursor := "  "
			if i == m.languageCursor {
				cursor = m.cursorStyle.Render("> ")
				b.WriteString(cursor)
				b.WriteString(m.selectedStyle.Render(opt))
			} else {
				b.WriteString(cursor)
				b.WriteString(opt)
			}
			b.WriteString("\n")
		}

	case stepTemplate:
		b.WriteString(m.promptStyle.Render("  Pick a workflow template"))
		b.WriteString("\n\n")
		for i, opt := range m.templateOptions {
			cursor := "  "
			if i == m.templateCursor {
				cursor = m.cursorStyle.Render("> ")
				b.WriteString(cursor)
				b.WriteString(m.selectedStyle.Render(opt))
			} else {
				b.WriteString(cursor)
				b.WriteString(opt)
			}
			b.WriteString("\n")
		}

	case stepRPCUrl:
		b.WriteString(m.promptStyle.Render("  Sepolia RPC URL"))
		b.WriteString("\n")
		b.WriteString(m.dimStyle.Render("  RPC endpoint for Ethereum Sepolia testnet"))
		b.WriteString("\n\n")
		b.WriteString("  ")
		b.WriteString(m.rpcInput.View())
		b.WriteString("\n")

	case stepWorkflowName:
		b.WriteString(m.promptStyle.Render("  Workflow name"))
		b.WriteString("\n")
		b.WriteString(m.dimStyle.Render("  Name for your workflow"))
		b.WriteString("\n\n")
		b.WriteString("  ")
		b.WriteString(m.workflowInput.View())
		b.WriteString("\n")

	case stepDone:
		// Nothing to render, wizard is complete
	}

	// Error message
	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorOrange500)).Render("  " + m.err))
		b.WriteString("\n")
	}

	// Help text
	b.WriteString("\n")
	if m.step == stepLanguage || m.step == stepTemplate {
		b.WriteString(m.helpStyle.Render("  ↑/↓ navigate • enter select • esc cancel"))
	} else {
		b.WriteString(m.helpStyle.Render("  enter confirm • esc cancel"))
	}
	b.WriteString("\n")

	return b.String()
}

func (m wizardModel) Result() WizardResult {
	return WizardResult{
		ProjectName:  m.projectName,
		Language:     m.language,
		TemplateName: m.templateName,
		RPCURL:       m.rpcURL,
		WorkflowName: m.workflowName,
		Completed:    m.completed,
		Cancelled:    m.cancelled,
	}
}

// RunWizard runs the interactive wizard and returns the result
func RunWizard(inputs Inputs, isNewProject bool, existingLanguage string) (WizardResult, error) {
	m := newWizardModel(inputs, isNewProject, existingLanguage)

	// Check if all steps are skipped
	if m.completed {
		return m.Result(), nil
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return WizardResult{}, err
	}

	result := finalModel.(wizardModel).Result()
	return result, nil
}

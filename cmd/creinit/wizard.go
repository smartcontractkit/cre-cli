package creinit

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/smartcontractkit/cre-cli/internal/constants"
	"github.com/smartcontractkit/cre-cli/internal/templaterepo"
	"github.com/smartcontractkit/cre-cli/internal/ui"
	"github.com/smartcontractkit/cre-cli/internal/validation"
)

const creLogo = `
      ÷÷÷                                          ÷÷÷
   ÷÷÷÷÷÷                                          ÷÷÷÷÷÷
÷÷÷÷÷÷÷÷÷                                          ÷÷÷÷÷÷÷÷÷
÷÷÷÷÷÷       ÷÷÷÷÷÷÷÷÷÷  ÷÷÷÷÷÷÷÷÷÷  ÷÷÷÷÷÷÷÷÷÷       ÷÷÷÷÷÷
÷÷÷÷÷÷       ÷÷÷÷÷÷÷÷÷÷  ÷÷÷÷÷÷÷÷÷÷  ÷÷÷÷÷÷÷÷÷÷       ÷÷÷÷÷÷
÷÷÷÷÷÷       ÷÷÷÷    ÷÷÷ ÷÷÷   ÷÷÷÷  ÷÷÷              ÷÷÷÷÷÷
÷÷÷÷÷÷       ÷÷÷         ÷÷÷÷÷÷÷÷÷   ÷÷÷÷÷÷÷÷÷÷       ÷÷÷÷÷÷
÷÷÷÷÷÷       ÷÷÷         ÷÷÷÷÷÷÷÷    ÷÷÷÷÷÷÷÷÷÷       ÷÷÷÷÷÷
÷÷÷÷÷÷       ÷÷÷÷    ÷÷÷ ÷÷÷  ÷÷÷÷   ÷÷÷              ÷÷÷÷÷÷
÷÷÷÷÷÷       ÷÷÷÷÷÷÷÷÷÷  ÷÷÷   ÷÷÷÷  ÷÷÷÷÷÷÷÷÷÷       ÷÷÷÷÷÷
÷÷÷÷÷÷       ÷÷÷÷÷÷÷÷÷÷  ÷÷÷    ÷÷÷÷ ÷÷÷÷÷÷÷÷÷÷       ÷÷÷÷÷÷
÷÷÷÷÷÷÷÷÷                                          ÷÷÷÷÷÷÷÷÷
   ÷÷÷÷÷÷                                          ÷÷÷÷÷÷
      ÷÷÷                                          ÷÷÷
`

type wizardStep int

const (
	stepProjectName wizardStep = iota
	stepTemplate
	stepNetworkRPCs
	stepWorkflowName
	stepDone
)

// wizardModel is the Bubble Tea model for the init wizard
type wizardModel struct {
	step wizardStep

	// Form values
	projectName  string
	workflowName string

	// Selected template
	selectedTemplate *templaterepo.TemplateSummary

	// Text inputs
	projectInput  textinput.Model
	workflowInput textinput.Model

	// Template list
	templates      []templaterepo.TemplateSummary
	templateCursor int
	filterText     string

	// RPC URL inputs
	networks        []string          // from selected template's Networks
	networkRPCs     map[string]string // chain-name -> url (collected results)
	rpcInputs       []textinput.Model // one text input per network
	rpcCursor       int               // which network RPC input is active
	skipNetworkRPCs bool              // skip if no networks or all RPCs provided via flags

	// Pre-provided RPC URLs from flags
	flagRpcURLs map[string]string

	// Flags to skip steps
	skipProjectName  bool
	skipTemplate     bool
	skipWorkflowName bool

	// Error message for validation
	err string

	// Whether wizard completed successfully
	completed bool
	cancelled bool

	// Styles
	logoStyle     lipgloss.Style
	titleStyle    lipgloss.Style
	dimStyle      lipgloss.Style
	promptStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	cursorStyle   lipgloss.Style
	helpStyle     lipgloss.Style
	tagStyle      lipgloss.Style
}

// WizardResult contains the wizard output
type WizardResult struct {
	ProjectName      string
	WorkflowName     string
	SelectedTemplate *templaterepo.TemplateSummary
	NetworkRPCs      map[string]string // chain-name -> rpc-url
	Completed        bool
	Cancelled        bool
}

func newWizardModel(inputs Inputs, isNewProject bool, templates []templaterepo.TemplateSummary, preselected *templaterepo.TemplateSummary) wizardModel {
	// Project name input
	pi := textinput.New()
	pi.Placeholder = constants.DefaultProjectName
	pi.CharLimit = 64
	pi.Width = 40

	// Workflow name input
	wi := textinput.New()
	wi.Placeholder = constants.DefaultWorkflowName
	wi.CharLimit = 64
	wi.Width = 40

	flagRPCs := inputs.RpcURLs
	if flagRPCs == nil {
		flagRPCs = make(map[string]string)
	}

	m := wizardModel{
		step:          stepProjectName,
		projectInput:  pi,
		workflowInput: wi,
		templates:     templates,
		flagRpcURLs:   flagRPCs,

		// Styles
		logoStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue500)).Bold(true),
		titleStyle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ui.ColorBlue500)),
		dimStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray500)),
		promptStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ui.ColorBlue400)),
		selectedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue500)),
		cursorStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue500)),
		helpStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray500)),
		tagStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray400)),
	}

	// Handle pre-populated values and skip flags
	if !isNewProject {
		m.skipProjectName = true
	}

	if inputs.ProjectName != "" {
		m.projectName = inputs.ProjectName
		m.skipProjectName = true
	}

	if preselected != nil {
		m.selectedTemplate = preselected
		m.skipTemplate = true
		m.initNetworkRPCInputs()
	}

	if inputs.WorkflowName != "" {
		m.workflowName = inputs.WorkflowName
		m.skipWorkflowName = true
	}

	// Start at the right step
	m.advanceToNextStep()

	return m
}

// initNetworkRPCInputs sets up RPC URL inputs based on the selected template's Networks.
func (m *wizardModel) initNetworkRPCInputs() {
	networks := m.selectedTemplate.Networks
	if len(networks) == 0 {
		m.skipNetworkRPCs = true
		return
	}

	m.networks = networks
	m.networkRPCs = make(map[string]string)
	m.rpcInputs = make([]textinput.Model, len(networks))

	allProvided := true
	for i, network := range networks {
		ti := textinput.New()
		ti.Placeholder = "https://..."
		ti.CharLimit = 256
		ti.Width = 60

		if rpcURL, ok := m.flagRpcURLs[network]; ok {
			m.networkRPCs[network] = rpcURL
		} else {
			allProvided = false
		}

		m.rpcInputs[i] = ti
	}

	if allProvided {
		m.skipNetworkRPCs = true
	}
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
		case stepTemplate:
			if m.skipTemplate {
				m.step++
				continue
			}
			return
		case stepNetworkRPCs:
			if m.skipNetworkRPCs {
				m.step++
				continue
			}
			// Focus the first unfilled RPC input
			for i, network := range m.networks {
				if _, ok := m.networkRPCs[network]; !ok {
					m.rpcCursor = i
					m.rpcInputs[i].Focus()
					return
				}
			}
			// All filled, advance
			m.step++
			continue
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

// filteredTemplates returns the templates that match the current filter text.
func (m *wizardModel) filteredTemplates() []templaterepo.TemplateSummary {
	if m.filterText == "" {
		return m.templates
	}
	filter := strings.ToLower(m.filterText)
	var filtered []templaterepo.TemplateSummary
	for _, t := range m.templates {
		if strings.Contains(strings.ToLower(t.Name), filter) ||
			strings.Contains(strings.ToLower(t.Title), filter) ||
			strings.Contains(strings.ToLower(t.Description), filter) ||
			strings.Contains(strings.ToLower(t.Language), filter) ||
			strings.Contains(strings.ToLower(t.Kind), filter) {
			filtered = append(filtered, t)
		}
		// Check tags
		for _, tag := range t.Tags {
			if strings.Contains(strings.ToLower(tag), filter) {
				filtered = append(filtered, t)
				break
			}
		}
	}
	// Remove duplicates from tag matching
	seen := make(map[string]bool)
	var unique []templaterepo.TemplateSummary
	for _, t := range filtered {
		if !seen[t.Name] {
			seen[t.Name] = true
			unique = append(unique, t)
		}
	}
	return unique
}

func (m wizardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.err = ""

		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.step == stepTemplate && m.templateCursor > 0 {
				m.templateCursor--
			}

		case "down", "j":
			if m.step == stepTemplate {
				filtered := m.filteredTemplates()
				if m.templateCursor < len(filtered)-1 {
					m.templateCursor++
				}
			}

		case "backspace":
			if m.step == stepTemplate && len(m.filterText) > 0 {
				m.filterText = m.filterText[:len(m.filterText)-1]
				m.templateCursor = 0
			}

		default:
			// Type-to-filter for template step
			if m.step == stepTemplate && len(msg.String()) == 1 {
				m.filterText += msg.String()
				m.templateCursor = 0
			}
		}
	}

	// Update text inputs
	var cmd tea.Cmd
	switch m.step {
	case stepProjectName:
		m.projectInput, cmd = m.projectInput.Update(msg)
	case stepWorkflowName:
		m.workflowInput, cmd = m.workflowInput.Update(msg)
	case stepNetworkRPCs:
		if m.rpcCursor < len(m.rpcInputs) {
			m.rpcInputs[m.rpcCursor], cmd = m.rpcInputs[m.rpcCursor].Update(msg)
		}
	case stepTemplate, stepDone:
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

	case stepTemplate:
		filtered := m.filteredTemplates()
		if len(filtered) == 0 {
			m.err = "No templates match your filter"
			return m, nil
		}
		if m.templateCursor >= len(filtered) {
			m.templateCursor = len(filtered) - 1
		}
		selected := filtered[m.templateCursor]
		m.selectedTemplate = &selected
		m.initNetworkRPCInputs()
		m.step++
		m.advanceToNextStep()

	case stepNetworkRPCs:
		value := strings.TrimSpace(m.rpcInputs[m.rpcCursor].Value())
		network := m.networks[m.rpcCursor]

		if value != "" {
			if err := validateRpcURL(value); err != nil {
				m.err = fmt.Sprintf("Invalid URL for %s: %s", network, err.Error())
				return m, nil
			}
			m.networkRPCs[network] = value
		}
		// Empty value means user skipped — leave blank

		if m.rpcCursor < len(m.networks)-1 {
			m.rpcInputs[m.rpcCursor].Blur()
			m.rpcCursor++
			m.rpcInputs[m.rpcCursor].Focus()
		} else {
			m.step++
			m.advanceToNextStep()
		}

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
		// Already done
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

	// Logo
	b.WriteString(m.logoStyle.Render(creLogo))
	b.WriteString("\n")

	// Title
	b.WriteString(m.titleStyle.Render("Create a new CRE project"))
	b.WriteString("\n\n")

	// History of completed steps
	if m.projectName != "" && m.step > stepProjectName {
		b.WriteString(m.dimStyle.Render("  Project: " + m.projectName))
		b.WriteString("\n")
	}
	if m.selectedTemplate != nil && m.step > stepTemplate {
		b.WriteString(m.dimStyle.Render("  Template: " + m.selectedTemplate.Title + " [" + m.selectedTemplate.Language + "]"))
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

	case stepTemplate:
		b.WriteString(m.promptStyle.Render("  Pick a template"))
		b.WriteString("\n")
		if m.filterText != "" {
			b.WriteString(m.dimStyle.Render("  Filter: " + m.filterText))
			b.WriteString("\n")
		} else {
			b.WriteString(m.dimStyle.Render("  Type to filter, ↑/↓ to navigate"))
			b.WriteString("\n")
		}
		b.WriteString("\n")

		filtered := m.filteredTemplates()

		// Group by kind
		var buildingBlocks, starterTemplates []templaterepo.TemplateSummary
		globalIdx := 0
		idxMap := make(map[int]int) // cursor index -> index in filtered

		for i, t := range filtered {
			if t.Kind == "building-block" {
				buildingBlocks = append(buildingBlocks, t)
			} else {
				starterTemplates = append(starterTemplates, t)
			}
			_ = i
		}

		// Render Building Blocks section
		if len(buildingBlocks) > 0 {
			b.WriteString(m.titleStyle.Render("  Building Blocks"))
			b.WriteString("\n")
			for _, t := range buildingBlocks {
				idxMap[globalIdx] = globalIdx
				cursor := "    "
				if globalIdx == m.templateCursor {
					cursor = m.cursorStyle.Render("  > ")
					b.WriteString(cursor)
					b.WriteString(m.selectedStyle.Render(t.Title))
				} else {
					b.WriteString(cursor)
					b.WriteString(t.Title)
				}
				b.WriteString(" ")
				b.WriteString(m.tagStyle.Render("[" + t.Language + "]"))
				b.WriteString("\n")
				if globalIdx == m.templateCursor && t.Description != "" {
					b.WriteString("      ")
					b.WriteString(m.dimStyle.Render(t.Description))
					b.WriteString("\n")
				}
				globalIdx++
			}
			b.WriteString("\n")
		}

		// Render Starter Templates section
		if len(starterTemplates) > 0 {
			b.WriteString(m.titleStyle.Render("  Starter Templates"))
			b.WriteString("\n")
			for _, t := range starterTemplates {
				idxMap[globalIdx] = globalIdx
				cursor := "    "
				if globalIdx == m.templateCursor {
					cursor = m.cursorStyle.Render("  > ")
					b.WriteString(cursor)
					b.WriteString(m.selectedStyle.Render(t.Title))
				} else {
					b.WriteString(cursor)
					b.WriteString(t.Title)
				}
				b.WriteString(" ")
				b.WriteString(m.tagStyle.Render("[" + t.Language + "]"))
				b.WriteString("\n")
				if globalIdx == m.templateCursor && t.Description != "" {
					b.WriteString("      ")
					b.WriteString(m.dimStyle.Render(t.Description))
					b.WriteString("\n")
				}
				globalIdx++
			}
		}

	case stepNetworkRPCs:
		b.WriteString(m.promptStyle.Render("  RPC URL overrides (optional)"))
		b.WriteString("\n")
		b.WriteString(m.dimStyle.Render("  The template has default RPC URLs. Press Enter to keep them, or type a URL to override."))
		b.WriteString("\n\n")

		for i, network := range m.networks {
			if i < m.rpcCursor {
				// Already answered
				rpcVal := m.networkRPCs[network]
				if rpcVal == "" {
					rpcVal = "(skipped)"
				}
				b.WriteString(m.dimStyle.Render(fmt.Sprintf("  %s: %s", network, rpcVal)))
				b.WriteString("\n")
			} else if i == m.rpcCursor {
				// Current input
				b.WriteString(m.promptStyle.Render(fmt.Sprintf("  %s", network)))
				b.WriteString("\n")
				b.WriteString("  ")
				b.WriteString(m.rpcInputs[i].View())
				b.WriteString("\n")
			}
		}

	case stepWorkflowName:
		b.WriteString(m.promptStyle.Render("  Workflow name"))
		b.WriteString("\n")
		b.WriteString(m.dimStyle.Render("  Name for your workflow"))
		b.WriteString("\n\n")
		b.WriteString("  ")
		b.WriteString(m.workflowInput.View())
		b.WriteString("\n")

	case stepDone:
		// Nothing to render
	}

	// Error message
	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorOrange500)).Render("  " + m.err))
		b.WriteString("\n")
	}

	// Help text
	b.WriteString("\n")
	if m.step == stepTemplate {
		b.WriteString(m.helpStyle.Render("  ↑/↓ navigate • type to filter • enter select • esc cancel"))
	} else {
		b.WriteString(m.helpStyle.Render("  enter confirm • esc cancel"))
	}
	b.WriteString("\n")

	return b.String()
}

func (m wizardModel) Result() WizardResult {
	return WizardResult{
		ProjectName:      m.projectName,
		WorkflowName:     m.workflowName,
		SelectedTemplate: m.selectedTemplate,
		NetworkRPCs:      m.networkRPCs,
		Completed:        m.completed,
		Cancelled:        m.cancelled,
	}
}

// RunWizard runs the interactive wizard and returns the result.
func RunWizard(inputs Inputs, isNewProject bool, templates []templaterepo.TemplateSummary, preselected *templaterepo.TemplateSummary) (WizardResult, error) {
	m := newWizardModel(inputs, isNewProject, templates, preselected)

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

// validateRpcURL validates that a URL is a valid HTTP/HTTPS URL.
func validateRpcURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must start with http:// or https://")
	}
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	return nil
}

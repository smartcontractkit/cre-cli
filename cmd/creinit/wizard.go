package creinit

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/list"
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

// templateItem wraps TemplateSummary for use with bubbles/list.
type templateItem struct {
	templaterepo.TemplateSummary
}

func (t templateItem) Title() string {
	if t.TemplateSummary.Title != "" {
		return t.TemplateSummary.Title
	}
	return t.TemplateSummary.Name
}
func (t templateItem) Description() string { return t.TemplateSummary.Description }
func (t templateItem) FilterValue() string {
	return t.TemplateSummary.Title + " " + t.TemplateSummary.Name + " " + t.TemplateSummary.Language
}

// languageFilter controls template list filtering by language.
type languageFilter int

const (
	filterAll languageFilter = iota
	filterGo
	filterTS
)

func (f languageFilter) String() string {
	switch f {
	case filterGo:
		return "Go"
	case filterTS:
		return "TypeScript"
	default:
		return "All"
	}
}

func (f languageFilter) next() languageFilter {
	switch f {
	case filterAll:
		return filterGo
	case filterGo:
		return filterTS
	default:
		return filterAll
	}
}

// sortTemplates sorts templates: built-in first, then by kind, then alphabetical by title.
func sortTemplates(templates []templaterepo.TemplateSummary) []templaterepo.TemplateSummary {
	sorted := slices.Clone(templates)
	slices.SortStableFunc(sorted, func(a, b templaterepo.TemplateSummary) int {
		// Built-in first
		if a.BuiltIn != b.BuiltIn {
			if a.BuiltIn {
				return -1
			}
			return 1
		}
		// Then by kind (building-block before starter-template)
		if a.Kind != b.Kind {
			return strings.Compare(a.Kind, b.Kind)
		}
		// Then alphabetical by title
		return strings.Compare(a.Title, b.Title)
	})
	return sorted
}

// templateDelegate is a custom list delegate that renders each template as:
//
//	Title  Go
//	Description line 1
//	Description line 2
type templateDelegate struct{}

func (d templateDelegate) Height() int                                         { return 3 }
func (d templateDelegate) Spacing() int                                        { return 1 }
func (d templateDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd            { return nil }
func (d templateDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	tmplItem, ok := item.(templateItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()
	isDimmed := m.FilterState() == list.Filtering && index != m.Index()

	title := stripLangSuffix(tmplItem.Title())
	lang := shortLang(tmplItem.TemplateSummary.Language)
	desc := tmplItem.Description()

	contentWidth := m.Width() - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	var (
		titleStyle lipgloss.Style
		descStyle  lipgloss.Style
		langStyle  lipgloss.Style
		prefix     string
	)

	borderChar := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue500)).Render("│")

	switch {
	case isSelected:
		prefix = borderChar + " "
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue500)).Bold(true)
		descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue300))
		langStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorTeal400)).Bold(true)
	case isDimmed:
		prefix = "  "
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray600))
		descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray700))
		langStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray700))
	default:
		prefix = "  "
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray50))
		descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray500))
		langStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray400))
	}

	// Line 1: title + language tag
	fmt.Fprintf(w, "%s%s %s", prefix, titleStyle.Render(title), langStyle.Render(lang))

	// Lines 2-3: description (word-wrapped, up to 2 lines)
	descLines := wrapText(desc, contentWidth)
	for i := 0; i < 2; i++ {
		fmt.Fprint(w, "\n")
		if i < len(descLines) {
			line := descLines[i]
			if i == 1 && len(descLines) > 2 {
				line += "..."
			}
			fmt.Fprintf(w, "%s%s", prefix, descStyle.Render(line))
		}
	}
}

// shortLang returns a short display label for a template language.
func shortLang(language string) string {
	switch strings.ToLower(language) {
	case "go":
		return "Go"
	case "typescript":
		return "TS"
	default:
		return language
	}
}

// stripLangSuffix removes trailing "(Go)" or "(TypeScript)" from a title.
func stripLangSuffix(title string) string {
	for _, suffix := range []string{" (Go)", " (TypeScript)", " (Typescript)"} {
		if strings.HasSuffix(title, suffix) {
			return strings.TrimSuffix(title, suffix)
		}
	}
	return title
}

// wrapText splits text into lines that fit within maxWidth, breaking at word boundaries.
func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > maxWidth {
			lines = append(lines, line)
			line = word
		} else {
			line += " " + word
		}
	}
	lines = append(lines, line)
	return lines
}

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
	templates    []templaterepo.TemplateSummary
	templateList list.Model
	langFilter   languageFilter

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

	// Directory existence check (inline overwrite confirmation)
	startDir         string // cwd, passed from Execute
	isNewProject     bool   // whether creating a new project
	dirExistsConfirm bool   // showing inline "overwrite?" prompt
	dirExistsYes     bool   // cursor position: true=Yes, false=No
	overwriteDir     bool   // user confirmed overwrite

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
	warnStyle     lipgloss.Style
}

// WizardResult contains the wizard output
type WizardResult struct {
	ProjectName      string
	WorkflowName     string
	SelectedTemplate *templaterepo.TemplateSummary
	NetworkRPCs      map[string]string // chain-name -> rpc-url
	OverwriteDir     bool              // user confirmed directory overwrite in wizard
	Completed        bool
	Cancelled        bool
}

func newWizardModel(inputs Inputs, isNewProject bool, startDir string, templates []templaterepo.TemplateSummary, preselected *templaterepo.TemplateSummary) wizardModel {
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

	// Build sorted template list items
	sorted := sortTemplates(templates)
	items := make([]list.Item, len(sorted))
	for i, t := range sorted {
		items[i] = templateItem{t}
	}

	tl := list.New(items, templateDelegate{}, 80, 20)
	tl.SetShowTitle(false)
	tl.SetShowStatusBar(false)
	tl.SetShowHelp(false)
	tl.SetFilteringEnabled(true)
	tl.Styles.NoItems = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray500)).Padding(0, 0, 0, 2)

	m := wizardModel{
		step:          stepProjectName,
		projectInput:  pi,
		workflowInput: wi,
		templates:     sorted,
		templateList:  tl,
		flagRpcURLs:   flagRPCs,
		startDir:      startDir,
		isNewProject:  isNewProject,

		// Styles
		logoStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue500)).Bold(true),
		titleStyle:    lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ui.ColorBlue500)),
		dimStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray500)),
		promptStyle:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ui.ColorBlue400)),
		selectedStyle: lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue500)),
		cursorStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue500)),
		helpStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray500)),
		tagStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGray400)),
		warnStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorOrange500)),
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
// It also configures workflow name behavior based on the template's Workflows field.
func (m *wizardModel) initNetworkRPCInputs() {
	// Multi-workflow templates: skip workflow name prompt (dirs are semantically meaningful)
	if len(m.selectedTemplate.Workflows) > 1 {
		m.skipWorkflowName = true
	}

	// Single workflow: use its dir name as the default placeholder
	if len(m.selectedTemplate.Workflows) == 1 {
		m.workflowInput.Placeholder = m.selectedTemplate.Workflows[0].Dir
	}

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

// rebuildTemplateItems filters m.templates by the current langFilter and updates the list.
func (m *wizardModel) rebuildTemplateItems() {
	var items []list.Item
	for _, t := range m.templates {
		if m.langFilter == filterAll ||
			(m.langFilter == filterGo && strings.EqualFold(t.Language, "go")) ||
			(m.langFilter == filterTS && strings.EqualFold(t.Language, "typescript")) {
			items = append(items, templateItem{t})
		}
	}
	m.templateList.SetItems(items)
}

func (m wizardModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if m.step == stepTemplate {
			m.templateList.SetWidth(msg.Width)
			// Reserve space for header (logo + title + tabs + help)
			m.templateList.SetHeight(max(msg.Height-24, 5))
		}
		return m, nil

	case tea.KeyMsg:
		m.err = ""

		// Template step: delegate most keys to the list
		if m.step == stepTemplate {
			switch msg.String() {
			case "ctrl+c":
				m.cancelled = true
				return m, tea.Quit
			case "esc":
				// If filtering, let list handle esc to cancel filter
				if m.templateList.FilterState() == list.Filtering {
					var cmd tea.Cmd
					m.templateList, cmd = m.templateList.Update(msg)
					return m, cmd
				}
				m.cancelled = true
				return m, tea.Quit
			case "tab":
				m.langFilter = m.langFilter.next()
				m.rebuildTemplateItems()
				return m, nil
			case "enter":
				return m.handleEnter(msg)
			default:
				// Delegate all other keys to the list (navigation, filtering, etc.)
				var cmd tea.Cmd
				m.templateList, cmd = m.templateList.Update(msg)
				return m, cmd
			}
		}

		// Non-template steps
		// Handle inline directory overwrite confirmation
		if m.dirExistsConfirm {
			switch msg.String() {
			case "ctrl+c":
				m.cancelled = true
				return m, tea.Quit
			case "esc":
				// Cancel the confirm, go back to editing
				m.dirExistsConfirm = false
				m.projectInput.Focus()
				return m, nil
			case "left", "right", "tab":
				m.dirExistsYes = !m.dirExistsYes
				return m, nil
			case "enter":
				if m.dirExistsYes {
					m.overwriteDir = true
					m.projectName = m.projectInput.Value()
					if m.projectName == "" {
						m.projectName = constants.DefaultProjectName
					}
					m.dirExistsConfirm = false
					m.step++
					m.advanceToNextStep()
					if m.completed {
						return m, tea.Quit
					}
					return m, nil
				}
				// User said No — go back to editing
				m.dirExistsConfirm = false
				m.projectInput.Focus()
				return m, nil
			default:
				// Any other key exits confirm and resumes typing
				m.dirExistsConfirm = false
				m.projectInput.Focus()
				var cmd tea.Cmd
				m.projectInput, cmd = m.projectInput.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
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
		// Handled above
	}

	return m, cmd
}

func (m wizardModel) handleEnter(msgs ...tea.Msg) (tea.Model, tea.Cmd) {
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
		// Check if the directory already exists (only for new projects)
		if m.isNewProject && m.startDir != "" && !m.overwriteDir {
			dirPath := filepath.Join(m.startDir, value)
			if _, statErr := os.Stat(dirPath); statErr == nil {
				m.dirExistsConfirm = true
				m.dirExistsYes = true
				m.projectInput.Blur()
				return m, nil
			}
		}
		m.projectName = value
		m.step++
		m.advanceToNextStep()

	case stepTemplate:
		// If the list is in filter mode, let it apply the filter
		if m.templateList.FilterState() == list.Filtering {
			if len(msgs) > 0 {
				var cmd tea.Cmd
				m.templateList, cmd = m.templateList.Update(msgs[0])
				return m, cmd
			}
			return m, nil
		}
		// Otherwise select the highlighted item
		selected, ok := m.templateList.SelectedItem().(templateItem)
		if !ok {
			m.err = "No template selected"
			return m, nil
		}
		tmpl := selected.TemplateSummary
		m.selectedTemplate = &tmpl
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
			if m.selectedTemplate != nil && len(m.selectedTemplate.Workflows) == 1 {
				value = m.selectedTemplate.Workflows[0].Dir
			} else {
				value = constants.DefaultWorkflowName
			}
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
		// Real-time validation hint
		if v := m.projectInput.Value(); v != "" && !m.dirExistsConfirm {
			if err := validation.IsValidProjectName(v); err != nil {
				b.WriteString(m.warnStyle.Render("  " + err.Error()))
				b.WriteString("\n")
			}
		}
		// Inline directory overwrite confirmation
		if m.dirExistsConfirm {
			value := m.projectInput.Value()
			if value == "" {
				value = constants.DefaultProjectName
			}
			dirPath := filepath.Join(m.startDir, value)
			b.WriteString("\n")
			b.WriteString(m.warnStyle.Render(fmt.Sprintf("  ⚠ Directory %s already exists. Overwrite?", dirPath)))
			b.WriteString("\n")
			yesLabel := "Yes"
			noLabel := "No"
			if m.dirExistsYes {
				yesLabel = m.selectedStyle.Render("[Yes]")
				noLabel = m.dimStyle.Render(" No ")
			} else {
				yesLabel = m.dimStyle.Render(" Yes ")
				noLabel = m.selectedStyle.Render("[No]")
			}
			b.WriteString(fmt.Sprintf("      %s  %s", yesLabel, noLabel))
			b.WriteString("\n")
		}

	case stepTemplate:
		b.WriteString(m.promptStyle.Render("  Pick a template"))
		b.WriteString("\n")

		// Language filter tabs
		tabs := []struct {
			filter languageFilter
			label  string
		}{
			{filterAll, "All"},
			{filterGo, "Go"},
			{filterTS, "TS"},
		}
		b.WriteString("  ")
		for i, tab := range tabs {
			if i > 0 {
				b.WriteString("  ")
			}
			if tab.filter == m.langFilter {
				b.WriteString(m.selectedStyle.Render("[" + tab.label + "]"))
			} else {
				b.WriteString(m.dimStyle.Render(" " + tab.label + " "))
			}
		}
		b.WriteString("\n\n")

		// Render the list
		b.WriteString(m.templateList.View())

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
				// Real-time validation hint for RPC URL
				if v := strings.TrimSpace(m.rpcInputs[i].Value()); v != "" {
					if err := validateRpcURL(v); err != nil {
						b.WriteString(m.warnStyle.Render("  " + err.Error()))
						b.WriteString("\n")
					}
				}
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
		// Real-time validation hint
		if v := m.workflowInput.Value(); v != "" {
			if err := validation.IsValidWorkflowName(v); err != nil {
				b.WriteString(m.warnStyle.Render("  " + err.Error()))
				b.WriteString("\n")
			}
		}

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
		b.WriteString(m.helpStyle.Render("  tab language filter • / search • ↑/↓ navigate • enter select • esc cancel"))
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
		OverwriteDir:     m.overwriteDir,
		Completed:        m.completed,
		Cancelled:        m.cancelled,
	}
}

// RunWizard runs the interactive wizard and returns the result.
func RunWizard(inputs Inputs, isNewProject bool, startDir string, templates []templaterepo.TemplateSummary, preselected *templaterepo.TemplateSummary) (WizardResult, error) {
	m := newWizardModel(inputs, isNewProject, startDir, templates, preselected)

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

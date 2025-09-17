//go:build windows

package prompt

import (
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type selectPromptModel struct {
	choices    []string
	cursor     int
	promptText string
	quitting   bool
}

func (m *selectPromptModel) Init() tea.Cmd { return nil }

func (m *selectPromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			m.quitting = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *selectPromptModel) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.promptText + "\n")
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		b.WriteString(cursor + " " + choice + "\n")
	}
	return b.String()
}

// SelectPrompt using Bubble Tea
func SelectPrompt(reader io.Reader, promptText string, choices []string, handler func(choice string) error) error {
	model := &selectPromptModel{
		choices:    choices,
		cursor:     0,
		promptText: promptText,
	}
	p := tea.NewProgram(model, tea.WithInput(reader))
	if _, err := p.Run(); err != nil {
		return err
	}
	return handler(model.choices[model.cursor])
}

// YesNoPrompt using Bubble Tea
func YesNoPrompt(reader io.Reader, promptText string) (bool, error) {
	choices := []string{"Yes", "No"}
	model := &selectPromptModel{
		choices:    choices,
		cursor:     0,
		promptText: promptText,
	}
	p := tea.NewProgram(model, tea.WithInput(reader))
	if _, err := p.Run(); err != nil {
		return false, err
	}
	return model.choices[model.cursor] == "Yes", nil
}

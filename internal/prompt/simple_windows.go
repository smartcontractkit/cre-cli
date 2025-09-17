//go:build windows

package prompt

import (
	"io"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type simplePromptModel struct {
	input      textinput.Model
	promptText string
	result     string
	quitting   bool
}

func (m *simplePromptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *simplePromptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.result = m.input.Value()
			m.quitting = true
			return m, tea.Quit
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *simplePromptModel) View() string {
	if m.quitting {
		return ""
	}
	return m.promptText + ": " + m.input.View()
}

// SimplePrompt using Bubble Tea
func SimplePrompt(reader io.Reader, promptText string, handler func(input string) error) error {
	input := textinput.New()
	input.Placeholder = promptText
	input.Focus()
	input.CharLimit = 256
	input.Width = 40

	model := &simplePromptModel{
		input:      input,
		promptText: promptText,
	}
	p := tea.NewProgram(model, tea.WithInput(reader))
	if _, err := p.Run(); err != nil {
		return err
	}
	return handler(model.result)
}

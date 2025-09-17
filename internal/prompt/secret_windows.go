//go:build windows

package prompt

import (
	"io"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// SecretPrompt using Bubble Tea
func SecretPrompt(reader io.Reader, promptText string, handler func(input string) error) error {
	input := textinput.New()
	input.Placeholder = promptText
	input.Focus()
	input.CharLimit = 256
	input.Width = 40
	input.EchoMode = textinput.EchoPassword
	input.EchoCharacter = '*'

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

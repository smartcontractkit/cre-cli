package ui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ChainlinkTheme returns a Huh theme using Chainlink Blocks palette
func ChainlinkTheme() *huh.Theme {
	t := huh.ThemeBase()

	// Focused state (when item is selected/active)
	t.Focused.Base = t.Focused.Base.BorderForeground(lipgloss.Color(ColorBlue500))
	t.Focused.Title = t.Focused.Title.Foreground(lipgloss.Color(ColorBlue400)).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(lipgloss.Color(ColorGray500))
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(lipgloss.Color(ColorBlue500))
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(lipgloss.Color(ColorBlue300))
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(lipgloss.Color(ColorGray500))
	t.Focused.FocusedButton = t.Focused.FocusedButton.
		Foreground(lipgloss.Color(ColorWhite)).
		Background(lipgloss.Color(ColorBlue600))
	t.Focused.BlurredButton = t.Focused.BlurredButton.
		Foreground(lipgloss.Color(ColorGray500)).
		Background(lipgloss.Color(ColorGray800))
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(lipgloss.Color(ColorBlue500))
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(lipgloss.Color(ColorGray500))
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(lipgloss.Color(ColorBlue500))

	// Blurred state (when not focused)
	t.Blurred.Base = t.Blurred.Base.BorderForeground(lipgloss.Color(ColorGray600))
	t.Blurred.Title = t.Blurred.Title.Foreground(lipgloss.Color(ColorGray500))
	t.Blurred.Description = t.Blurred.Description.Foreground(lipgloss.Color(ColorGray600))
	t.Blurred.SelectSelector = t.Blurred.SelectSelector.Foreground(lipgloss.Color(ColorGray600))
	t.Blurred.SelectedOption = t.Blurred.SelectedOption.Foreground(lipgloss.Color(ColorGray500))
	t.Blurred.UnselectedOption = t.Blurred.UnselectedOption.Foreground(lipgloss.Color(ColorGray600))

	return t
}

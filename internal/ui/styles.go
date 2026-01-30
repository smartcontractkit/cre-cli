package ui

import "github.com/charmbracelet/lipgloss"

// Styles - centralized styling for consistent CLI appearance
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginBottom(1)

	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("10"))

	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("9"))

	WarningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("11"))

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("12")).
			Padding(0, 1)

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	StepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14"))

	BoldStyle = lipgloss.NewStyle().
			Bold(true)

	CodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)
)

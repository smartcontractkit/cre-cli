package ui

import "fmt"

// Output helpers - use these for consistent styled output across commands.
// These functions make it easy to migrate from raw fmt.Println calls.

// Title prints a styled title/header (high visibility - Chainlink Blue)
func Title(text string) {
	fmt.Println(TitleStyle.Render(text))
}

// Success prints a success message with checkmark (Green)
func Success(text string) {
	fmt.Println(SuccessStyle.Render("✓ " + text))
}

// Error prints an error message (Orange - high contrast)
func Error(text string) {
	fmt.Println(ErrorStyle.Render("✗ " + text))
}

// Warning prints a warning message (Yellow)
func Warning(text string) {
	fmt.Println(WarningStyle.Render("! " + text))
}

// Dim prints dimmed/secondary text (Gray - less important)
func Dim(text string) {
	fmt.Println(DimStyle.Render("  " + text))
}

// Step prints a step instruction (Light Blue - visible)
func Step(text string) {
	fmt.Println(StepStyle.Render(text))
}

// Command prints a CLI command (Bold Light Blue - prominent)
func Command(text string) {
	fmt.Println(CommandStyle.Render(text))
}

// Box prints text in a bordered box (Chainlink Blue border)
func Box(text string) {
	fmt.Println(BoxStyle.Render(text))
}

// Bold prints bold text
func Bold(text string) {
	fmt.Println(BoldStyle.Render(text))
}

// Code prints text styled as code (Light Blue)
func Code(text string) {
	fmt.Println(CodeStyle.Render(text))
}

// URL prints a styled URL (Chainlink Blue, underlined)
func URL(text string) {
	fmt.Println(URLStyle.Render(text))
}

// Line prints an empty line
func Line() {
	fmt.Println()
}

// Print prints plain text (for gradual migration - can be replaced later)
func Print(text string) {
	fmt.Println(text)
}

// Printf prints formatted plain text
func Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// Indent returns text with indentation
func Indent(text string, level int) string {
	indent := ""
	for i := 0; i < level; i++ {
		indent += "  "
	}
	return indent + text
}

// Render functions - return styled string without printing (for composition)

func RenderTitle(text string) string {
	return TitleStyle.Render(text)
}

func RenderSuccess(text string) string {
	return SuccessStyle.Render(text)
}

func RenderError(text string) string {
	return ErrorStyle.Render(text)
}

func RenderWarning(text string) string {
	return WarningStyle.Render(text)
}

func RenderDim(text string) string {
	return DimStyle.Render(text)
}

func RenderStep(text string) string {
	return StepStyle.Render(text)
}

func RenderBold(text string) string {
	return BoldStyle.Render(text)
}

func RenderCode(text string) string {
	return CodeStyle.Render(text)
}

func RenderCommand(text string) string {
	return CommandStyle.Render(text)
}

func RenderURL(text string) string {
	return URLStyle.Render(text)
}

func RenderAccent(text string) string {
	return AccentStyle.Render(text)
}

package ui

import (
	"github.com/charmbracelet/huh"
)

// --- Option types for functional options pattern ---

// ConfirmOption configures a Confirm prompt.
type ConfirmOption func(*confirmConfig)

type confirmConfig struct {
	affirmative string
	negative    string
	description string
}

// WithLabels sets custom affirmative/negative button labels for Confirm.
func WithLabels(affirmative, negative string) ConfirmOption {
	return func(c *confirmConfig) {
		c.affirmative = affirmative
		c.negative = negative
	}
}

// WithDescription sets the description text for a prompt.
func WithDescription(desc string) ConfirmOption {
	return func(c *confirmConfig) {
		c.description = desc
	}
}

// Confirm displays a yes/no confirmation prompt and returns the user's choice.
func Confirm(title string, opts ...ConfirmOption) (bool, error) {
	cfg := confirmConfig{}
	for _, o := range opts {
		o(&cfg)
	}

	var result bool
	confirm := huh.NewConfirm().
		Title(title).
		Value(&result)

	if cfg.affirmative != "" {
		confirm = confirm.Affirmative(cfg.affirmative)
	}
	if cfg.negative != "" {
		confirm = confirm.Negative(cfg.negative)
	}
	if cfg.description != "" {
		confirm = confirm.Description(cfg.description)
	}

	form := huh.NewForm(
		huh.NewGroup(confirm),
	).WithTheme(ChainlinkTheme())

	if err := form.Run(); err != nil {
		return false, err
	}
	return result, nil
}

// --- Input ---

// InputOption configures an Input prompt.
type InputOption func(*inputConfig)

type inputConfig struct {
	description string
	placeholder string
}

// WithInputDescription sets the description for an Input prompt.
func WithInputDescription(desc string) InputOption {
	return func(c *inputConfig) {
		c.description = desc
	}
}

// WithPlaceholder sets the placeholder text for an Input prompt.
func WithPlaceholder(placeholder string) InputOption {
	return func(c *inputConfig) {
		c.placeholder = placeholder
	}
}

// Input displays a single text input prompt and returns the entered value.
func Input(title string, opts ...InputOption) (string, error) {
	cfg := inputConfig{}
	for _, o := range opts {
		o(&cfg)
	}

	var result string
	input := huh.NewInput().
		Title(title).
		Value(&result)

	if cfg.description != "" {
		input = input.Description(cfg.description)
	}
	if cfg.placeholder != "" {
		input = input.Placeholder(cfg.placeholder)
	}

	form := huh.NewForm(
		huh.NewGroup(input),
	).WithTheme(ChainlinkTheme())

	if err := form.Run(); err != nil {
		return "", err
	}
	return result, nil
}

// --- Select ---

// SelectOption represents a single option in a Select prompt.
type SelectOption[T comparable] struct {
	Label string
	Value T
}

// Select displays a selection prompt and returns the chosen value.
func Select[T comparable](title string, options []SelectOption[T]) (T, error) {
	var result T

	huhOpts := make([]huh.Option[T], len(options))
	for i, opt := range options {
		huhOpts[i] = huh.NewOption(opt.Label, opt.Value)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[T]().
				Title(title).
				Options(huhOpts...).
				Value(&result),
		),
	).WithTheme(ChainlinkTheme())

	if err := form.Run(); err != nil {
		return result, err
	}
	return result, nil
}

// --- InputForm (multi-field) ---

// InputField represents a single field in a multi-field InputForm.
type InputField struct {
	Title       string
	Description string
	Placeholder string
	Value       *string
	Validate    func(string) error
	Suggestions []string
}

// InputForm displays a multi-field input form. Each field writes to its Value pointer.
func InputForm(fields []InputField) error {
	huhFields := make([]huh.Field, len(fields))
	for i, f := range fields {
		input := huh.NewInput().
			Title(f.Title).
			Value(f.Value)

		if f.Description != "" {
			input = input.Description(f.Description)
		}
		if f.Placeholder != "" {
			input = input.Placeholder(f.Placeholder)
		}
		if f.Validate != nil {
			input = input.Validate(f.Validate)
		}
		if len(f.Suggestions) > 0 {
			input = input.Suggestions(f.Suggestions)
		}
		huhFields[i] = input
	}

	form := huh.NewForm(
		huh.NewGroup(huhFields...),
	).WithTheme(ChainlinkTheme()).WithKeyMap(ChainlinkKeyMap())

	return form.Run()
}

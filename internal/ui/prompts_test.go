package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithLabels(t *testing.T) {
	cfg := confirmConfig{}
	opt := WithLabels("Accept", "Decline")
	opt(&cfg)

	assert.Equal(t, "Accept", cfg.affirmative)
	assert.Equal(t, "Decline", cfg.negative)
}

func TestWithDescription(t *testing.T) {
	cfg := confirmConfig{}
	opt := WithDescription("Some description")
	opt(&cfg)

	assert.Equal(t, "Some description", cfg.description)
}

func TestWithInputDescription(t *testing.T) {
	cfg := inputConfig{}
	opt := WithInputDescription("Input desc")
	opt(&cfg)

	assert.Equal(t, "Input desc", cfg.description)
}

func TestWithPlaceholder(t *testing.T) {
	cfg := inputConfig{}
	opt := WithPlaceholder("Enter value...")
	opt(&cfg)

	assert.Equal(t, "Enter value...", cfg.placeholder)
}

func TestSelectOptionStruct(t *testing.T) {
	opts := []SelectOption[int]{
		{Label: "Option A", Value: 1},
		{Label: "Option B", Value: 2},
	}

	assert.Equal(t, "Option A", opts[0].Label)
	assert.Equal(t, 1, opts[0].Value)
	assert.Equal(t, "Option B", opts[1].Label)
	assert.Equal(t, 2, opts[1].Value)
}

func TestSelectOptionStringType(t *testing.T) {
	opts := []SelectOption[string]{
		{Label: "Go", Value: "golang"},
		{Label: "TS", Value: "typescript"},
	}

	assert.Equal(t, "golang", opts[0].Value)
	assert.Equal(t, "typescript", opts[1].Value)
}

func TestInputFieldStruct(t *testing.T) {
	var val string
	field := InputField{
		Title:       "Test",
		Description: "A test field",
		Placeholder: "placeholder",
		Value:       &val,
		Validate: func(s string) error {
			return nil
		},
		Suggestions: []string{"suggestion1"},
	}

	assert.Equal(t, "Test", field.Title)
	assert.Equal(t, "A test field", field.Description)
	assert.Equal(t, "placeholder", field.Placeholder)
	assert.NotNil(t, field.Value)
	assert.NotNil(t, field.Validate)
	assert.NoError(t, field.Validate("anything"))
	assert.Equal(t, []string{"suggestion1"}, field.Suggestions)
}

func TestConfirmOptionsCompose(t *testing.T) {
	cfg := confirmConfig{}
	opts := []ConfirmOption{
		WithLabels("Yes", "No"),
		WithDescription("Are you sure?"),
	}
	for _, o := range opts {
		o(&cfg)
	}

	assert.Equal(t, "Yes", cfg.affirmative)
	assert.Equal(t, "No", cfg.negative)
	assert.Equal(t, "Are you sure?", cfg.description)
}

func TestInputOptionsCompose(t *testing.T) {
	cfg := inputConfig{}
	opts := []InputOption{
		WithInputDescription("desc"),
		WithPlaceholder("ph"),
	}
	for _, o := range opts {
		o(&cfg)
	}

	assert.Equal(t, "desc", cfg.description)
	assert.Equal(t, "ph", cfg.placeholder)
}

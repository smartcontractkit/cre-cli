package validation

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

// Define custom translation messages as constants for reuse
const (
	CustomUppercaseMessage = "{0} must be in uppercase letters only!: {1}"
	CustomRequiredMessage  = "{0} is mandatory!"
)

// TestNewValidator_Success tests that NewValidator initializes correctly.
func TestNewValidator_Success(t *testing.T) {
	v, err := NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")
	assert.NotNil(t, v, "Validator instance should not be nil")
	assert.NotNil(t, v.validate, "Validator.validate should not be nil")
	assert.NotNil(t, v.trans, "Validator.trans should not be nil")
}

func TestValidator(t *testing.T) {
	type Foo struct {
		Name  string `validate:"required,uppercase"`
		Age   int    `validate:"required"`
		Email string `validate:"required,email" cli:"--email"`
	}

	// Define a slice of test cases
	tests := []struct {
		name             string
		setup            func(*Validator) error
		input            interface{}
		wantError        bool
		wantErrorKeys    []string
		wantErrorDetails []string
	}{
		{
			name: "RegisterCustomTranslation_Success",
			setup: func(v *Validator) error {
				return v.RegisterCustomTranslation("uppercase", CustomUppercaseMessage)
			},
			input: &Foo{
				Name:  "hello",
				Age:   10,
				Email: "test@example.com",
			},
			wantError:        true,
			wantErrorKeys:    []string{"Foo.Name"},
			wantErrorDetails: []string{"Name must be in uppercase letters only!: hello"},
		},
		{
			name: "Struct_Valid",
			setup: func(v *Validator) error {
				// No custom translations needed
				return nil
			},
			input: &Foo{
				Name:  "HELLO_WORLD",
				Age:   10,
				Email: "test@example.com",
			},
			wantError: false,
		},
		{
			name: "Struct_MultipleErrors",
			setup: func(v *Validator) error {
				// Register multiple custom translations
				if err := v.RegisterCustomTranslation("required", CustomRequiredMessage); err != nil {
					return err
				}
				return v.RegisterCustomTranslation("uppercase", CustomUppercaseMessage)
			},
			input: &Foo{
				Name:  "hello",
				Age:   0,
				Email: "test@example.com",
			},
			wantError:        true,
			wantErrorKeys:    []string{"Foo.Name", "Foo.Age"},
			wantErrorDetails: []string{"Name must be in uppercase letters only!: hello", "Age is mandatory!"},
		},
		{
			name: "Struct_CLIOverwriteError",
			setup: func(v *Validator) error {
				return nil
			},
			input: &Foo{
				Name: "HELLO_WORLD",
				Age:  10,
			},
			wantError:        true,
			wantErrorKeys:    []string{"Foo.Email"},
			wantErrorDetails: []string{"--email is a required field"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the validator for each subtest to avoid state leakage
			v, err := NewValidator()
			assert.NoError(t, err, "Expected no error during validator initialization")
			assert.NotNil(t, v, "Validator instance should not be nil")

			// Execute the setup function for custom translations
			if tt.setup != nil {
				err := tt.setup(v)
				assert.NoError(t, err, "Setup function returned an error")
			}

			err = v.Struct(tt.input)

			if tt.wantError {
				assert.Error(t, err, "Expected validation error")

				// Ensure the error is wrapped and of type ValidationErrors
				var verrs validator.ValidationErrors
				assert.True(t, errors.As(err, &verrs), "Expected error to wrap validator.ValidationErrors")

				for i := range tt.wantErrorKeys {
					AssertErrors(t, err, tt.wantErrorKeys[i], tt.wantErrorDetails[i], v)
				}
			} else {
				assert.NoError(t, err, "Did not expect validation error")
			}
		})
	}

	t.Run("Parses multiple validation errors", func(t *testing.T) {
		input := &Foo{
			Name:  "invalid name",
			Age:   10,
			Email: "invalid_email",
		}

		v, err := NewValidator()
		assert.NoError(t, err, "Expected no error during validator initialization")

		err = v.Struct(input)
		assert.Error(t, err, "Expected validation error")

		validationErr := v.ParseValidationErrors(err)
		validationErrStr := fmt.Sprintf("%v", validationErr)
		assert.Equal(t, "validation error\nName must be an uppercase string\n--email must be a valid email address\n", validationErrStr)
	})
}

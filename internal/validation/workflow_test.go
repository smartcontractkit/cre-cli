package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateWorkflowOwner(t *testing.T) {
	validator, err := NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	type WorkflowTestStruct struct {
		WorkflowOwner string `validate:"workflow_owner"`
	}

	tests := []struct {
		name            string
		input           WorkflowTestStruct
		wantError       bool
		wantErrorKey    string
		wantErrorDetail string
	}{
		{
			name: "Valid Ethereum address",
			input: WorkflowTestStruct{
				WorkflowOwner: "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
			},
			wantError: false,
		},
		{
			name: "Invalid Ethereum address - missing 0x prefix",
			input: WorkflowTestStruct{
				WorkflowOwner: "f39fd6e51aad88f6f4ce6ab8827279cfffb92266",
			},
			wantError:       true,
			wantErrorKey:    "WorkflowTestStruct.WorkflowOwner",
			wantErrorDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: f39fd6e51aad88f6f4ce6ab8827279cfffb92266",
		},
		{
			name: "Invalid Ethereum address - incorrect length",
			input: WorkflowTestStruct{
				WorkflowOwner: "0x12345",
			},
			wantError:       true,
			wantErrorKey:    "WorkflowTestStruct.WorkflowOwner",
			wantErrorDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: 0x12345",
		},
		{
			name: "Empty Ethereum address",
			input: WorkflowTestStruct{
				WorkflowOwner: "",
			},
			wantError:       true,
			wantErrorKey:    "WorkflowTestStruct.WorkflowOwner",
			wantErrorDetail: "WorkflowOwner must be a valid Ethereum address starting with 0x: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = validator.Struct(tt.input)

			if !tt.wantError && err != nil {
				assert.NoError(t, err, "test should not result in errors")
			}

			if tt.wantError {
				assert.Error(t, err, "test should result in error")
				AssertErrors(t, err, tt.wantErrorKey, tt.wantErrorDetail, validator)
			}
		})
	}
}

func TestValidateWorkflowName(t *testing.T) {
	validator, err := NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	type WorkflowTestStruct struct {
		WorkflowName string `validate:"workflow_name"`
	}

	tests := []struct {
		name            string
		input           WorkflowTestStruct
		wantError       bool
		wantErrorKey    string
		wantErrorDetail string
	}{
		{
			name: "Valid Workflow Name",
			input: WorkflowTestStruct{
				WorkflowName: "valid_workflow_name",
			},
			wantError: false,
		},
		{
			name: "Empty Workflow Name",
			input: WorkflowTestStruct{
				WorkflowName: "",
			},
			wantError:       true,
			wantErrorKey:    "WorkflowTestStruct.WorkflowName",
			wantErrorDetail: "WorkflowName must be non-empty and no longer than 64 characters: ",
		},
		{
			name: "Workflow Name exceeds max length",
			input: WorkflowTestStruct{
				WorkflowName: "this_is_a_really_long_workflow_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
			},
			wantError:       true,
			wantErrorKey:    "WorkflowTestStruct.WorkflowName",
			wantErrorDetail: "WorkflowName must be non-empty and no longer than 64 characters: this_is_a_really_long_workflow_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = validator.Struct(tt.input)

			if !tt.wantError && err != nil {
				assert.NoError(t, err, "test should not result in errors")
			}

			if tt.wantError {
				assert.Error(t, err, "test should result in error")
				AssertErrors(t, err, tt.wantErrorKey, tt.wantErrorDetail, validator)
			}
		})
	}
}

func TestWorkflowOwnerPanicOnNonString(t *testing.T) {
	validator, err := NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	type InvalidTypeStruct struct {
		WorkflowOwner int `validate:"workflow_owner"`
	}

	input := InvalidTypeStruct{WorkflowOwner: 12345}

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r, "input field name is not a string", "Expected panic for non-string argument")
		} else {
			t.Error("Expected panic, but did not get one")
		}
	}()

	_ = validator.Struct(input)
}

func TestWorkflowNamePanicOnNonString(t *testing.T) {
	validator, err := NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	type InvalidTypeStruct struct {
		WorkflowName int `validate:"workflow_name"`
	}

	input := InvalidTypeStruct{WorkflowName: 12345}

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r, "input field name is not a string", "Expected panic for non-string argument")
		} else {
			t.Error("Expected panic, but did not get one")
		}
	}()

	_ = validator.Struct(input)
}

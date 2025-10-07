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
			wantErrorDetail: "WorkflowName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): ",
		},
		{
			name: "Workflow Name exceeds max length",
			input: WorkflowTestStruct{
				WorkflowName: "this_is_a_really_long_workflow_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
			},
			wantError:       true,
			wantErrorKey:    "WorkflowTestStruct.WorkflowName",
			wantErrorDetail: "WorkflowName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): this_is_a_really_long_workflow_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
		},
		{
			name: "Workflow Name with invalid characters - spaces",
			input: WorkflowTestStruct{
				WorkflowName: "invalid workflow name",
			},
			wantError:       true,
			wantErrorKey:    "WorkflowTestStruct.WorkflowName",
			wantErrorDetail: "WorkflowName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): invalid workflow name",
		},
		{
			name: "Workflow Name with invalid characters - special chars",
			input: WorkflowTestStruct{
				WorkflowName: "invalid@workflow#name",
			},
			wantError:       true,
			wantErrorKey:    "WorkflowTestStruct.WorkflowName",
			wantErrorDetail: "WorkflowName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): invalid@workflow#name",
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

func TestValidateProjectName(t *testing.T) {
	validator, err := NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	type ProjectTestStruct struct {
		ProjectName string `validate:"project_name"`
	}

	tests := []struct {
		name            string
		input           ProjectTestStruct
		wantError       bool
		wantErrorKey    string
		wantErrorDetail string
	}{
		{
			name: "Valid Project Name",
			input: ProjectTestStruct{
				ProjectName: "valid-project_name123",
			},
			wantError: false,
		},
		{
			name: "Empty Project Name",
			input: ProjectTestStruct{
				ProjectName: "",
			},
			wantError:       true,
			wantErrorKey:    "ProjectTestStruct.ProjectName",
			wantErrorDetail: "ProjectName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): ",
		},
		{
			name: "Project Name exceeds max length",
			input: ProjectTestStruct{
				ProjectName: "this_is_a_really_long_project_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
			},
			wantError:       true,
			wantErrorKey:    "ProjectTestStruct.ProjectName",
			wantErrorDetail: "ProjectName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): this_is_a_really_long_project_name_that_exceeds_the_maximum_allowed_length_of_64_chars",
		},
		{
			name: "Project Name with invalid characters - spaces",
			input: ProjectTestStruct{
				ProjectName: "invalid project name",
			},
			wantError:       true,
			wantErrorKey:    "ProjectTestStruct.ProjectName",
			wantErrorDetail: "ProjectName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): invalid project name",
		},
		{
			name: "Project Name with invalid characters - special chars",
			input: ProjectTestStruct{
				ProjectName: "invalid@project#name",
			},
			wantError:       true,
			wantErrorKey:    "ProjectTestStruct.ProjectName",
			wantErrorDetail: "ProjectName must be non-empty, no longer than 64 characters, and contain only letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_): invalid@project#name",
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

func TestProjectNamePanicOnNonString(t *testing.T) {
	validator, err := NewValidator()
	assert.NoError(t, err, "Expected no error during validator initialization")

	type InvalidTypeStruct struct {
		ProjectName int `validate:"project_name"`
	}

	input := InvalidTypeStruct{ProjectName: 12345}

	defer func() {
		if r := recover(); r != nil {
			assert.Contains(t, r, "input field name is not a string", "Expected panic for non-string argument")
		} else {
			t.Error("Expected panic, but did not get one")
		}
	}()

	_ = validator.Struct(input)
}

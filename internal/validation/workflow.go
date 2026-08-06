package validation

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-playground/validator/v10"
)

const (
	maxWorkflowNameLength = 64
	maxProjectNameLength  = 64
	maxOwnerLabelLength   = 64
)

// ValidNameRegex matches only letters (upper and lower case), numbers, dashes, and underscores
var ValidNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidOwnerLabelRegex matches human-readable labels: letters, numbers, spaces, dots, dashes and
// underscores, and must start with a letter or number. Quotes, shell metacharacters, path
// separators, control characters and non-ASCII bytes are all excluded so that a label can never
// break out of the YAML scalar it is substituted into, nor be mistaken for a flag or a path.
var ValidOwnerLabelRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9 ._-]*$`)

func isWorkflowOwner(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}

	address := fl.Field().String()
	return common.IsHexAddress(address) && address[:2] == "0x"
}

func isProjectName(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}

	projectName := fl.Field().String()
	return IsValidProjectName(projectName) == nil
}

func isWorkflowName(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}

	workflowName := fl.Field().String()
	return IsValidWorkflowName(workflowName) == nil
}

func isOwnerLabel(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}

	ownerLabel := fl.Field().String()
	return IsValidOwnerLabel(ownerLabel) == nil
}

func IsValidProjectName(projectName string) error {
	if projectName == "" {
		return fmt.Errorf("project name can't be an empty string")
	}

	if len(projectName) > maxProjectNameLength {
		return fmt.Errorf("project name is too long, limit is %d characters", maxProjectNameLength)
	}

	if !ValidNameRegex.MatchString(projectName) {
		return fmt.Errorf("project name can only contain letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_)")
	}

	return nil
}

func IsValidWorkflowName(workflowName string) error {
	if workflowName == "" {
		return fmt.Errorf("workflow name can't be an empty string")
	}

	if len(workflowName) > maxWorkflowNameLength {
		return fmt.Errorf("workflow name is too long, limit is %d characters", maxWorkflowNameLength)
	}

	if !ValidNameRegex.MatchString(workflowName) {
		return fmt.Errorf("workflow name can only contain letters (a-z, A-Z), numbers (0-9), dashes (-), and underscores (_)")
	}

	return nil
}

func IsValidOwnerLabel(ownerLabel string) error {
	if ownerLabel == "" {
		return fmt.Errorf("owner label can't be an empty string")
	}

	if len(ownerLabel) > maxOwnerLabelLength {
		return fmt.Errorf("owner label is too long, limit is %d characters", maxOwnerLabelLength)
	}

	if !ValidOwnerLabelRegex.MatchString(ownerLabel) {
		return fmt.Errorf("owner label must start with a letter or number and can only contain letters (a-z, A-Z), numbers (0-9), spaces, dots (.), dashes (-), and underscores (_)")
	}

	return nil
}

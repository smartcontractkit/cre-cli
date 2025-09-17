package validation

import (
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-playground/validator/v10"
)

const maxWorkflowNameLength = 64

func isWorkflowOwner(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}

	address := fl.Field().String()
	return common.IsHexAddress(address) && address[:2] == "0x"
}

func isWorkflowName(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() != reflect.String {
		panic(fmt.Sprintf("input field name is not a string: %s", fl.FieldName()))
	}

	workflowName := fl.Field().String()
	return IsValidWorkflowName(workflowName) == nil
}

func IsValidWorkflowName(workflowName string) error {
	if workflowName == "" {
		return fmt.Errorf("workflow name can't be an empty string")
	}

	if len(workflowName) > maxWorkflowNameLength {
		return fmt.Errorf("workflow name is too long, limit is %d characters", maxWorkflowNameLength)
	}
	return nil
}

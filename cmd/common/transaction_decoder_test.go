package common

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatDecodedTxInputs(t *testing.T) {
	rawInputMap := map[string]interface{}{
		"WorkflowID":    [32]byte{1, 2, 3}, // Partial bytes
		"SampleData":    [3]byte{1, 2},     // Partial bytes
		"WorkflowName":  "Sample Workflow",
		"WorkflowOwner": "0x1234567890abcdef1234567890abcdef12345678",

		"DonID": 12345,
	}
	txInput := NewDecodedTxInputs(rawInputMap)

	result := fmt.Sprint(txInput)

	expected := `map[DonID:12345 SampleData:010200 WorkflowID:0102030000000000000000000000000000000000000000000000000000000000 WorkflowName:Sample Workflow WorkflowOwner:0x1234567890abcdef1234567890abcdef12345678]`
	assert.Equal(t, expected, result)
}

func TestFormatDecodedTxInputsArraySlice(t *testing.T) {
	rawInputMap := map[string]interface{}{
		"allowedWorkflowNames": [][10]byte{{1, 2, 3}, {1, 2, 4}}, // Partial bytes
	}
	txInput := NewDecodedTxInputs(rawInputMap)

	result := fmt.Sprint(txInput)

	expected := `map[allowedWorkflowNames:[01020300000000000000 01020400000000000000]]`
	assert.Equal(t, expected, result)
}

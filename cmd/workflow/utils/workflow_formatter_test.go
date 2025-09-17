package workflow

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"

	workflow_registry_wrapper "github.com/smartcontractkit/chainlink-evm/gethwrappers/workflow/generated/workflow_registry_wrapper_v2"
)

func TestFormatWorkflowsTable_EmptyList(t *testing.T) {
	var metadata []workflow_registry_wrapper.WorkflowRegistryWorkflowMetadataView

	expected := `No workflow available for owner`

	result := FormatWorkflowsTable(metadata)
	assert.Equal(t, expected, result)
}

func TestFormatWorkflowsTable_SingleWorkflow_Active(t *testing.T) {
	metadata := []workflow_registry_wrapper.WorkflowRegistryWorkflowMetadataView{
		{
			WorkflowName: "Test Workflow",
			WorkflowId:   [32]byte{1, 2, 3},
			Owner:        common.HexToAddress("0x123"),
			BinaryUrl:    "http://binary.url",
			ConfigUrl:    "http://config.url",
			DonFamily:    "1",
			Status:       0,
		},
	}

	expected := `
+---------------+------------------------------------------------------------------+-----+--------+
| NAME          | ID                                                               | DON | STATUS |
+---------------+------------------------------------------------------------------+-----+--------+
| Test Workflow | 0102030000000000000000000000000000000000000000000000000000000000 |  1  | [1;32mActive[1;37m |
+---------------+------------------------------------------------------------------+-----+--------+`
	trimmedExpected := strings.TrimPrefix(expected, "\n")

	result := FormatWorkflowsTable(metadata)
	assert.Equal(t, trimmedExpected, result)
}

func TestFormatWorkflowsTable_SingleWorkflow_Paused(t *testing.T) {
	metadata := []workflow_registry_wrapper.WorkflowRegistryWorkflowMetadataView{
		{
			WorkflowName: "Paused Workflow",
			WorkflowId:   [32]byte{4, 5, 6},
			Owner:        common.HexToAddress("0x456"),
			BinaryUrl:    "http://binary.url",
			ConfigUrl:    "http://config.url",
			DonFamily:    "2",
			Status:       1,
		},
	}

	expected := `
+-----------------+------------------------------------------------------------------+-----+--------+
| NAME            | ID                                                               | DON | STATUS |
+-----------------+------------------------------------------------------------------+-----+--------+
| Paused Workflow | 0405060000000000000000000000000000000000000000000000000000000000 |  2  | [1;33mPaused[1;37m |
+-----------------+------------------------------------------------------------------+-----+--------+`
	trimmedExpected := strings.TrimPrefix(expected, "\n")

	result := FormatWorkflowsTable(metadata)
	assert.Equal(t, trimmedExpected, result)
}

func TestFormatWorkflowsTable_MultipleWorkflows_OrderedByName(t *testing.T) {
	metadata := []workflow_registry_wrapper.WorkflowRegistryWorkflowMetadataView{
		{
			WorkflowName: "Workflow B",
			WorkflowId:   [32]byte{1, 2, 3},
			Owner:        common.HexToAddress("0x123"),
			BinaryUrl:    "http://binary.url/b",
			ConfigUrl:    "http://config.url/b",
			DonFamily:    "1",
			Status:       0,
		},
		{
			WorkflowName: "Workflow A",
			WorkflowId:   [32]byte{4, 5, 6},
			Owner:        common.HexToAddress("0x456"),
			BinaryUrl:    "http://binary.url/a",
			ConfigUrl:    "http://config.url/a",
			DonFamily:    "2",
			Status:       1,
		},
		{
			WorkflowName: "Workflow C",
			WorkflowId:   [32]byte{7, 8, 9},
			Owner:        common.HexToAddress("0x789"),
			BinaryUrl:    "http://binary.url/c",
			ConfigUrl:    "http://config.url/c",
			DonFamily:    "3",
			Status:       0,
		},
	}

	expected := `
+------------+------------------------------------------------------------------+-----+--------+
| NAME       | ID                                                               | DON | STATUS |
+------------+------------------------------------------------------------------+-----+--------+
| Workflow A | 0405060000000000000000000000000000000000000000000000000000000000 |  2  | [1;33mPaused[1;37m |
| Workflow B | 0102030000000000000000000000000000000000000000000000000000000000 |  1  | [1;32mActive[1;37m |
| Workflow C | 0708090000000000000000000000000000000000000000000000000000000000 |  3  | [1;32mActive[1;37m |
+------------+------------------------------------------------------------------+-----+--------+`
	trimmedExpected := strings.TrimPrefix(expected, "\n")

	result := FormatWorkflowsTable(metadata)
	assert.Equal(t, trimmedExpected, result)
}

func TestFormatWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		metadata workflow_registry_wrapper.WorkflowRegistryWorkflowMetadataView
		expected string
	}{
		{
			name: "Active Workflow",
			metadata: workflow_registry_wrapper.WorkflowRegistryWorkflowMetadataView{
				WorkflowName: "Active Workflow",
				WorkflowId:   [32]byte{1, 2, 3},
				Owner:        common.HexToAddress("0x123"),
				BinaryUrl:    "http://binary.url",
				ConfigUrl:    "http://config.url",
				DonFamily:    "1",
				Status:       0,
			},
			expected: `
Workflow Metadata:
  * Name: Active Workflow
  * ID: 0102030000000000000000000000000000000000000000000000000000000000
  * Status: [1;32mActive[1;37m
  * DON: 1
  * Owner: 0x0000000000000000000000000000000000000123
  * Binary URL: http://binary.url
  * Config URL: http://config.url
`,
		},
		{
			name: "Paused Workflow",
			metadata: workflow_registry_wrapper.WorkflowRegistryWorkflowMetadataView{
				WorkflowName: "Paused Workflow",
				WorkflowId:   [32]byte{4, 5, 6},
				Owner:        common.HexToAddress("0x456"),
				BinaryUrl:    "http://binary.url",
				ConfigUrl:    "",
				DonFamily:    "2",
				Status:       1,
			},
			expected: `
Workflow Metadata:
  * Name: Paused Workflow
  * ID: 0405060000000000000000000000000000000000000000000000000000000000
  * Status: [1;33mPaused[1;37m
  * DON: 2
  * Owner: 0x0000000000000000000000000000000000000456
  * Binary URL: http://binary.url
  * Config URL: (None provided)
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatWorkflow(tt.metadata)
			trimmedExpected := strings.TrimPrefix(tt.expected, "\n")
			assert.Equal(t, trimmedExpected, result)
		})
	}
}

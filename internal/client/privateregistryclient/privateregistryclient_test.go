package privateregistryclient

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateUpsertWorkflowInput(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		err := validateUpsertWorkflowInput(WorkflowInRegistryInput{
			WorkflowID:   "wf-1",
			Status:       WorkflowStatusActive,
			WorkflowName: "test-workflow",
			BinaryURL:    "s3://binary",
			DonFamily:    "family-a",
		})
		require.NoError(t, err)
	})

	tests := []struct {
		name  string
		input WorkflowInRegistryInput
		err   string
	}{
		{
			name:  "missing workflowId",
			input: WorkflowInRegistryInput{Status: WorkflowStatusActive, WorkflowName: "w", BinaryURL: "b", DonFamily: "f"},
			err:   "workflowId is required",
		},
		{
			name:  "missing status",
			input: WorkflowInRegistryInput{WorkflowID: "wf", WorkflowName: "w", BinaryURL: "b", DonFamily: "f"},
			err:   "status is required",
		},
		{
			name:  "missing workflowName",
			input: WorkflowInRegistryInput{WorkflowID: "wf", Status: WorkflowStatusActive, BinaryURL: "b", DonFamily: "f"},
			err:   "workflowName is required",
		},
		{
			name:  "missing binaryUrl",
			input: WorkflowInRegistryInput{WorkflowID: "wf", Status: WorkflowStatusActive, WorkflowName: "w", DonFamily: "f"},
			err:   "binaryUrl is required",
		},
		{
			name:  "missing donFamily",
			input: WorkflowInRegistryInput{WorkflowID: "wf", Status: WorkflowStatusActive, WorkflowName: "w", BinaryURL: "b"},
			err:   "donFamily is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUpsertWorkflowInput(tc.input)
			require.Error(t, err)
			assert.Equal(t, tc.err, err.Error())
		})
	}
}

func TestCreateServiceContextWithTimeout(t *testing.T) {
	logger := zerolog.Nop()
	client := New(nil, &logger)
	client.SetServiceTimeout(150 * time.Millisecond)

	ctx, cancel := client.CreateServiceContextWithTimeout()
	defer cancel()

	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(150*time.Millisecond), deadline, 100*time.Millisecond)
}

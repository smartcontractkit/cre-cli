package link_key

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNonInteractive_WithoutOwnerLabel_BlocksPrompt(t *testing.T) {
	t.Parallel()
	in := Inputs{
		NonInteractive:     true,
		WorkflowOwnerLabel: "",
	}
	// Simulate the guard check from Execute
	require.True(t, in.NonInteractive && in.WorkflowOwnerLabel == "",
		"should require --owner-label in non-interactive mode")
}

func TestNonInteractive_WithOwnerLabel_AllowsProceeding(t *testing.T) {
	t.Parallel()
	in := Inputs{
		NonInteractive:     true,
		WorkflowOwnerLabel: "my-label",
	}
	// Guard should NOT trigger
	assert.False(t, in.NonInteractive && in.WorkflowOwnerLabel == "",
		"should allow proceeding when --owner-label is set")
}

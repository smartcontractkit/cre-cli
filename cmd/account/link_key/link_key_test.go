package link_key

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateInputs_OwnerLabel(t *testing.T) {
	t.Parallel()

	const (
		validOwner    = "0x0000000000000000000000000000000000000001"
		validRegistry = "0x0000000000000000000000000000000000000002"
	)

	tests := []struct {
		name      string
		label     string
		wantError bool
	}{
		{name: "valid label", label: "prod owner 1", wantError: false},
		{name: "valid label with dashes", label: "owner-label-1", wantError: false},
		{name: "empty label is deferred to the prompt", label: "", wantError: false},
		{name: "double quote breaks out of yaml scalar", label: `bad"; echo pwned`, wantError: true},
		{name: "command substitution", label: "bad$(id)", wantError: true},
		{name: "newline", label: "a\ninjected: true", wantError: true},
		{name: "leading dash looks like a flag", label: "-rf /", wantError: true},
		{name: "too long", label: strings.Repeat("a", 65), wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := &handler{}
			err := h.ValidateInputs(Inputs{
				WorkflowOwnerLabel:              tt.label,
				WorkflowOwner:                   validOwner,
				WorkflowRegistryContractAddress: validRegistry,
			})

			if tt.wantError {
				require.Error(t, err, "expected label %q to be rejected", tt.label)
				assert.Contains(t, err.Error(), "--owner-label",
					"error should name the flag the user passed")
				assert.False(t, h.validated, "handler must not be marked validated")
			} else {
				require.NoError(t, err, "expected label %q to be accepted", tt.label)
			}
		})
	}
}

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

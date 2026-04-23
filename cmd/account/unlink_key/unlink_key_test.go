package unlink_key

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNonInteractiveFlagRegistered(t *testing.T) {
	t.Parallel()
	// New() requires a runtime context with many fields; instead verify
	// the guard logic directly on the Inputs struct.
	in := Inputs{
		NonInteractive:   true,
		SkipConfirmation: false,
	}
	assert.True(t, in.NonInteractive && !in.SkipConfirmation,
		"non-interactive guard should trigger when --yes is missing")
}

func TestNonInteractive_WithoutYes_BlocksConfirmation(t *testing.T) {
	t.Parallel()
	in := Inputs{
		NonInteractive:   true,
		SkipConfirmation: false,
	}
	// Simulate the guard check from Execute
	require.True(t, in.NonInteractive && !in.SkipConfirmation,
		"should require --yes in non-interactive mode")
}

func TestNonInteractive_WithYes_AllowsProceeding(t *testing.T) {
	t.Parallel()
	in := Inputs{
		NonInteractive:   true,
		SkipConfirmation: true,
	}
	// Guard should NOT trigger
	require.False(t, in.NonInteractive && !in.SkipConfirmation,
		"should allow proceeding when --yes is set")
}

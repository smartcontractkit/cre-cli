package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

)

// TestErrorOutput_UnknownCommand verifies that running an unknown command
// produces an error message on stderr and exits with a non-zero code.
// This guards against regressions from SilenceErrors: true in root.go.
func TestErrorOutput_UnknownCommand(t *testing.T) {
	isolatedEnv(t)
	res, err := runCLI(t, []string{"nonexistent-command"})
	require.Error(t, err, "expected non-zero exit code for unknown command")

	assert.Contains(t, res.Stderr, "unknown command", "expected 'unknown command' error on stderr, got:\nSTDOUT: %s\nSTDERR: %s", res.Stdout, res.Stderr)
	assert.NotContains(t, res.Stdout, "unknown command", "error message should be on stderr, not stdout")
}

// TestErrorOutput_UnknownFlag verifies that an unknown flag produces an
// error message on stderr and exits with a non-zero code.
func TestErrorOutput_UnknownFlag(t *testing.T) {
	isolatedEnv(t)
	res, err := runCLI(t, []string{"--nonexistent-flag"})
	require.Error(t, err, "expected non-zero exit code for unknown flag")

	assert.Contains(t, res.Stderr, "unknown flag", "expected 'unknown flag' error on stderr, got:\nSTDOUT: %s\nSTDERR: %s", res.Stdout, res.Stderr)
	assert.NotContains(t, res.Stdout, "unknown flag", "error message should be on stderr, not stdout")
}

// TestErrorOutput_MissingRequiredArg verifies that a subcommand requiring
// an argument produces an error on stderr when called without one.
func TestErrorOutput_MissingRequiredArg(t *testing.T) {
	isolatedEnv(t)
	res, err := runCLI(t, []string{"workflow", "simulate"})
	require.Error(t, err, "expected non-zero exit code for missing required arg")

	assert.NotEmpty(t, res.Stderr, "expected error output on stderr, got nothing.\nSTDOUT: %s", res.Stdout)
}

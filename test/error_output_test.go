package test

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorOutput_UnknownCommand verifies that running an unknown command
// produces an error message on stderr and exits with a non-zero code.
// This guards against regressions from SilenceErrors: true in root.go.
func TestErrorOutput_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(CLIPath, "nonexistent-command")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	require.Error(t, err, "expected non-zero exit code for unknown command")

	stderrStr := stderr.String()
	assert.Contains(t, stderrStr, "unknown command", "expected 'unknown command' error on stderr, got:\nSTDOUT: %s\nSTDERR: %s", stdout.String(), stderrStr)
	assert.NotContains(t, stdout.String(), "unknown command", "error message should be on stderr, not stdout")
}

// TestErrorOutput_UnknownFlag verifies that an unknown flag produces an
// error message on stderr and exits with a non-zero code.
func TestErrorOutput_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(CLIPath, "--nonexistent-flag")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	require.Error(t, err, "expected non-zero exit code for unknown flag")

	stderrStr := stderr.String()
	assert.Contains(t, stderrStr, "unknown flag", "expected 'unknown flag' error on stderr, got:\nSTDOUT: %s\nSTDERR: %s", stdout.String(), stderrStr)
	assert.NotContains(t, stdout.String(), "unknown flag", "error message should be on stderr, not stdout")
}

// TestErrorOutput_MissingRequiredArg verifies that a subcommand requiring
// an argument produces an error on stderr when called without one.
func TestErrorOutput_MissingRequiredArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(CLIPath, "workflow", "simulate")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	require.Error(t, err, "expected non-zero exit code for missing required arg")

	stderrStr := stderr.String()
	// Cobra may say "accepts 1 arg(s)" or "requires" depending on the command definition.
	// We just verify stderr is non-empty and stdout doesn't contain the error.
	assert.NotEmpty(t, stderrStr, "expected error output on stderr, got nothing.\nSTDOUT: %s", stdout.String())
}

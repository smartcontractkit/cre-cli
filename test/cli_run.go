package test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/testutil/cretest"
)

// isolatedEnv sets up an isolated CRE config directory for integration tests.
func isolatedEnv(t *testing.T) *cretest.Env {
	t.Helper()
	return cretest.NewEnv(t)
}

// runCLI executes the cre binary with isolated CRE_CONFIG_DIR.
func runCLI(t *testing.T, args []string, opts ...cretest.RunOption) (cretest.Result, error) {
	t.Helper()
	return cretest.RunCLI(t, CLIPath, args, opts...)
}

// requireCLI runs the cre binary and fails the test on non-zero exit.
func requireCLI(t *testing.T, msg string, args []string, opts ...cretest.RunOption) cretest.Result {
	t.Helper()
	res, err := runCLI(t, args, opts...)
	require.NoError(t, err, "%s:\nSTDOUT:\n%s\nSTDERR:\n%s", msg, res.Stdout, res.Stderr)
	return res
}

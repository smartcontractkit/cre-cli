package multi_command_flows

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/testutil/cretest"
)

func runCLI(t *testing.T, args []string, opts ...cretest.RunOption) (cretest.Result, error) {
	t.Helper()
	return cretest.RunCLI(t, "", args, opts...)
}

func requireCLI(t *testing.T, msg string, args []string, opts ...cretest.RunOption) cretest.Result {
	t.Helper()
	res, err := runCLI(t, args, opts...)
	require.NoError(t, err, "%s:\nSTDOUT:\n%s\nSTDERR:\n%s", msg, res.Stdout, res.Stderr)
	return res
}

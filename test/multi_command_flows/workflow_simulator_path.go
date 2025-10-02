package multi_command_flows

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// Simulates a workflow
func RunSimulationHappyPath(t *testing.T, tc TestConfig) {
	t.Helper()

	t.Run("Simulate", func(t *testing.T) {
		// Build CLI args
		args := []string{
			"workflow", "simulate",
			"chainreader_workflow",
			tc.GetCliEnvFlag(),
			tc.GetProjectRootFlag(),
			"--non-interactive",
			"--trigger-index=0",
		}

		cmd := exec.Command(CLIPath, args...)

		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr

		require.NoError(
			t,
			cmd.Run(),
			"cre workflow simulation failed:\nSTDOUT:\n%s\nSTDERR:\n%s",
			stdout.String(),
			stderr.String(),
		)

		out := StripANSI(stdout.String() + stderr.String())

		require.Contains(t, out, "Workflow compiled", "expected workflow to compile.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "Simulator Initialized", "expected workflow to initialize.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "Getting native balances", "expected workflow to read from balance reader.\nCLI OUTPUT:\n%s", out)
		require.Contains(t, out, "Workflow Simulation Result", "expected simulation success.\nCLI OUTPUT:\n%s", out)
	})
}

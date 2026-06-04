package test

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/testutil/cretest"
)

func convertSimulateCaptureOutput(t *testing.T, projectRoot, workflowName string) string {
	t.Helper()
	res := requireCLI(t, "simulate (before convert) failed",
		[]string{"workflow", "simulate", workflowName,
			"--project-root", projectRoot,
			"--non-interactive", "--trigger-index=0",
			"--target=staging-settings",
		},
		cretest.WithDir(projectRoot),
	)
	return res.Stdout
}

func convertSimulateRequireOutputContains(t *testing.T, projectRoot, workflowName, expectedSubstring string) {
	t.Helper()
	res := requireCLI(t, "simulate (after convert) failed",
		[]string{"workflow", "simulate", workflowName,
			"--project-root", projectRoot,
			"--non-interactive", "--trigger-index=0",
			"--target=staging-settings",
		},
		cretest.WithDir(projectRoot),
	)
	require.Contains(t, res.Stdout, expectedSubstring,
		"simulate output after convert should contain %q", expectedSubstring)
}

// ConvertSimulateBeforeAfter runs simulate (capture output), convert, then simulate again
// and verifies output contains the same expectedSubstring. Simulate runs make build internally when needed.
func ConvertSimulateBeforeAfter(t *testing.T, projectRoot, workflowDir, workflowName, expectedSubstring string) {
	t.Helper()
	beforeOutput := convertSimulateCaptureOutput(t, projectRoot, workflowName)
	require.Contains(t, beforeOutput, expectedSubstring,
		"baseline simulate output should contain %q", expectedSubstring)
	convertRunConvert(t, projectRoot, workflowDir)
	convertSimulateRequireOutputContains(t, projectRoot, workflowName, expectedSubstring)
}

func convertRunConvert(t *testing.T, projectRoot, workflowDir string) {
	t.Helper()
	requireCLI(t, "convert failed",
		[]string{"workflow", "custom-build", workflowDir, "-f"},
		cretest.WithDir(projectRoot),
	)
}

func convertRunMakeBuild(t *testing.T, workflowDir string, makeArgs ...string) {
	t.Helper()
	args := []string{"build"}
	args = append(args, makeArgs...)
	cmd := exec.Command("make", args...)
	cmd.Dir = workflowDir
	require.NoError(t, cmd.Run(), "make build failed")
}

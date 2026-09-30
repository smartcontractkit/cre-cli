package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildCommandPath constructs a minimal cobra command hierarchy so that
// cmd.CommandPath() returns the desired space-separated path string.
// The leaf (last segment) is returned.
func buildCommandPath(path ...string) *cobra.Command {
	root := &cobra.Command{Use: path[0]}
	parent := root
	for _, seg := range path[1:] {
		child := &cobra.Command{Use: seg}
		parent.AddCommand(child)
		parent = child
	}
	return parent
}

func TestIsRegistryRPCCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    []string
		wantRPC bool
	}{
		// Workflow lifecycle — must validate RPC (on-chain or nil registry).
		{"workflow deploy", []string{"cre", "workflow", "deploy"}, true},
		{"workflow pause", []string{"cre", "workflow", "pause"}, true},
		{"workflow activate", []string{"cre", "workflow", "activate"}, true},
		{"workflow delete", []string{"cre", "workflow", "delete"}, true},
		// Account key commands — always on-chain.
		{"account link-key", []string{"cre", "account", "link-key"}, true},
		{"account unlink-key", []string{"cre", "account", "unlink-key"}, true},
		// Other commands — must NOT trigger RPC validation.
		{"workflow simulate", []string{"cre", "workflow", "simulate"}, false},
		{"secrets create", []string{"cre", "secrets", "create"}, false},
		{"secrets list", []string{"cre", "secrets", "list"}, false},
		{"root cre", []string{"cre"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := buildCommandPath(tc.path...)
			assert.Equal(t, tc.wantRPC, isRegistryRPCCommand(cmd),
				"isRegistryRPCCommand(%q)", cmd.CommandPath())
		})
	}
}

func TestWorkflowTriggerHTTP_HiddenFromHelpButWired(t *testing.T) {
	workflowCmd, _, err := RootCmd.Find([]string{"workflow"})
	require.NoError(t, err)

	var help bytes.Buffer
	workflowCmd.SetOut(&help)
	t.Cleanup(func() { workflowCmd.SetOut(nil) })
	require.NoError(t, workflowCmd.Help())
	assert.Contains(t, help.String(), "simulate", "workflow help should still render its commands")
	assert.NotContains(t, help.String(), "trigger", "trigger must stay out of `cre workflow --help`")

	httpCmd, _, err := RootCmd.Find([]string{"workflow", "trigger", "http"})
	require.NoError(t, err)
	assert.Equal(t, "cre workflow trigger http", httpCmd.CommandPath())
	assert.True(t, httpCmd.Hidden)
	assert.True(t, httpCmd.Parent().Hidden)
}

func TestWorkflowTriggerHTTP_NeedsLoginAndWorkflowSettings(t *testing.T) {
	httpCmd := buildCommandPath("cre", "workflow", "trigger", "http")

	assert.True(t, isLoadCredentials(httpCmd), "workflow lookup and the tenant gateway URL need a logged-in session")
	assert.True(t, isLoadSettings(httpCmd), "the target, workflow name and wallet key come from workflow settings")
	assert.False(t, isRegistryRPCCommand(httpCmd), "triggering must not require a registry RPC URL")
}

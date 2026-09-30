package trigger_test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/workflow/trigger"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

func TestNew_CommandsAreHiddenButRunnable(t *testing.T) {
	triggerCmd := trigger.New(&runtime.Context{})

	assert.True(t, triggerCmd.Hidden, "trigger group must not be advertised")

	httpCmd, _, err := triggerCmd.Find([]string{"http"})
	require.NoError(t, err)
	require.Equal(t, "http", httpCmd.Name())
	assert.True(t, httpCmd.Hidden, "http subcommand must not be advertised")
	assert.NotNil(t, httpCmd.RunE, "hidden must not mean disabled")
	assert.Error(t, httpCmd.Args(httpCmd, nil), "the workflow folder argument is required")
	assert.NoError(t, httpCmd.Args(httpCmd, []string{"./my-workflow"}))
}

func TestNew_ParentHelpDoesNotListTrigger(t *testing.T) {
	workflowCmd := &cobra.Command{Use: "workflow", Short: "Manages workflows"}
	workflowCmd.AddCommand(&cobra.Command{Use: "deploy", Short: "Deploys a workflow", Run: func(*cobra.Command, []string) {}})
	workflowCmd.AddCommand(trigger.New(&runtime.Context{}))

	var help bytes.Buffer
	workflowCmd.SetOut(&help)
	require.NoError(t, workflowCmd.Help())

	assert.Contains(t, help.String(), "deploy")
	assert.NotContains(t, help.String(), "trigger")
}

func TestNew_HTTPFlags(t *testing.T) {
	httpCmd, _, err := trigger.New(&runtime.Context{}).Find([]string{"http"})
	require.NoError(t, err)

	input := httpCmd.Flags().Lookup("input")
	require.NotNil(t, input)
	assert.Equal(t, "{}", input.DefValue)
	for _, name := range []string{"workflow-id", "output", "json"} {
		assert.NotNil(t, httpCmd.Flags().Lookup(name), "missing --%s", name)
	}
}

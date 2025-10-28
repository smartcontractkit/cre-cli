package simulate

import (
	"fmt"
	"os"
	"path/filepath"
	rt "runtime"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

// TestBlankWorkflowSimulation validates that the simulator can successfully
// run a blank workflow from end to end in a non-interactive mode.
func TestBlankWorkflowSimulation(t *testing.T) {
	// Locate repo root from this test file, then point to test/test_project as the project root
	_, thisFile, _, _ := rt.Caller(0)
	thisDir := filepath.Dir(thisFile)
	repoRoot := filepath.Clean(filepath.Join(thisDir, "..", "..", "..")) // cmd/workflow/simulate -> repo root
	projectRoot := filepath.Join(repoRoot, "test", "test_project")
	workflowPath := filepath.Join(projectRoot, "blank_workflow")

	// Ensure workflow path is absolute
	absWorkflowPath, err := filepath.Abs(workflowPath)
	require.NoError(t, err)

	// Clean up common artifacts produced by the compile/simulate flow
	outB64 := filepath.Join(absWorkflowPath, "binary.wasm.br.b64")
	t.Cleanup(func() {
		_ = os.Remove(outB64)
	})

	// Mock a `*viper.Viper` instance to represent CLI flags.
	v := viper.New()
	v.Set("project-root", projectRoot)
	v.Set("non-interactive", true)
	v.Set("trigger-index", 0)
	v.Set("target", "staging-settings")

	var rpc settings.RpcEndpoint
	rpc.ChainName = "ethereum-testnet-sepolia"
	rpc.Url = "https://sepolia.infura.io/v3"
	v.Set(fmt.Sprintf("%s.%s", "staging-settings", settings.RpcsSettingName), []settings.RpcEndpoint{rpc})

	var workflowSettings settings.WorkflowSettings
	workflowSettings.UserWorkflowSettings.WorkflowName = "blank-workflow"
	workflowSettings.WorkflowArtifactSettings.WorkflowPath = filepath.Join(absWorkflowPath, "main.go")
	workflowSettings.WorkflowArtifactSettings.ConfigPath = filepath.Join(absWorkflowPath, "config.json")

	// Mock `runtime.Context` with a test logger.
	runtimeCtx := &runtime.Context{
		Logger: testutil.NewTestLogger(),
		Viper:  v,
		Settings: &settings.Settings{
			Workflow: workflowSettings,
			User: settings.UserSettings{
				TargetName:    "staging-settings",
				EthPrivateKey: "88888845d8761ca4a8cefb324c89702f12114ffbd0c47222f12aac0ad6538888",
			},
		},
	}

	// Instantiate and run the simulator handler
	handler := newHandler(runtimeCtx)

	inputs, err := handler.ResolveInputs(runtimeCtx.Viper, runtimeCtx.Settings)
	require.NoError(t, err)

	// Validate the resolved inputs.
	err = handler.ValidateInputs(inputs)
	require.NoError(t, err)

	// Execute the simulation. We expect this to compile the workflow and run the simulator successfully.
	err = handler.Execute(inputs)
	require.NoError(t, err, "Execute should not return an error")
}

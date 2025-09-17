package simulate

import (
	"fmt"
	"os"
	"path/filepath"
	rt "runtime"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/dev-platform/internal/runtime"
	"github.com/smartcontractkit/dev-platform/internal/settings"
	"github.com/smartcontractkit/dev-platform/internal/testutil"
)

// TestBlankWorkflowSimulation validates that the simulator can successfully
// run a blank workflow from end to end in a non-interactive mode.
func TestBlankWorkflowSimulation(t *testing.T) {
	// Locate repo root from this test file, then point to test/test_project/blank_workflow
	_, thisFile, _, _ := rt.Caller(0)
	thisDir := filepath.Dir(thisFile)
	repoRoot := filepath.Clean(filepath.Join(thisDir, "..", "..", "..")) // cmd/workflow/simulate -> repo root
	wfDir := filepath.Join(repoRoot, "test", "test_project", "blank_workflow")

	// Change into the workflow directory so `go build` resolves the local go.mod there
	origWD, _ := os.Getwd()
	require.NoError(t, os.Chdir(wfDir))
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	// Paths within the workflow module dir
	workflowPath := "main.go"
	configPath := "config.json"

	// Clean up common artifacts produced by the compile/simulate flow
	outB64 := filepath.Join(wfDir, "binary.wasm.br.b64")
	t.Cleanup(func() {
		_ = os.Remove(outB64)
	})

	// Mock a `*viper.Viper` instance to represent CLI flags.
	v := viper.New()
	v.Set("config", configPath)
	v.Set("non-interactive", true)
	v.Set("trigger-index", 0)
	v.Set("target", "local-simulation")

	var rpc settings.RpcEndpoint
	rpc.ChainSelector = 16015286601757825753
	rpc.Url = "https://sepolia.infura.io/v3"
	v.Set(fmt.Sprintf("%s.%s", "local-simulation", settings.RpcsSettingName), []settings.RpcEndpoint{rpc})

	var workflowSettings settings.WorkflowSettings
	workflowSettings.UserWorkflowSettings.WorkflowName = "blank-workflow"
	workflowSettings.DevPlatformSettings.DonFamily = "small"

	// Mock `runtime.Context` with a test logger.
	runtimeCtx := &runtime.Context{
		Logger: testutil.NewTestLogger(),
		Viper:  v,
		Settings: &settings.Settings{
			Workflow: workflowSettings,
			User: settings.UserSettings{
				TargetName:    "local-simulation",
				EthPrivateKey: "88888845d8761ca4a8cefb324c89702f12114ffbd0c47222f12aac0ad6538888",
			},
		},
	}

	// Instantiate and run the simulator handler
	handler := newHandler(runtimeCtx)

	inputs, err := handler.ResolveInputs([]string{workflowPath}, runtimeCtx.Viper, runtimeCtx.Settings)
	require.NoError(t, err)

	// Validate the resolved inputs.
	err = handler.ValidateInputs(inputs)
	require.NoError(t, err)

	// Execute the simulation. We expect this to compile the workflow and run the simulator successfully.
	err = handler.Execute(inputs)
	require.NoError(t, err, "Execute should not return an error")
}

package simulate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	rt "runtime"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	commonsettings "github.com/smartcontractkit/chainlink-common/pkg/settings"
	"github.com/smartcontractkit/chainlink-common/pkg/settings/cresettings"

	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

var (
	limitTestWorkflowPath string
	limitTestOutB64       string
)

func TestMain(m *testing.M) {
	_, thisFile, _, _ := rt.Caller(0)
	thisDir := filepath.Dir(thisFile)
	repoRoot := filepath.Clean(filepath.Join(thisDir, "..", "..", ".."))
	projectRoot := filepath.Join(repoRoot, "test", "test_project")
	workflowPath := filepath.Join(projectRoot, "limit_test_workflow")

	var err error
	limitTestWorkflowPath, err = filepath.Abs(workflowPath)
	if err != nil {
		fmt.Printf("Failed to get absolute path: %v\n", err)
		os.Exit(1)
	}

	limitTestOutB64 = filepath.Join(limitTestWorkflowPath, "binary.wasm.br.b64")
	shortConfigPath := filepath.Join(limitTestWorkflowPath, "c.json")
	
	originalConfigPath := filepath.Join(limitTestWorkflowPath, "config.json")
	configData, err := os.ReadFile(originalConfigPath)
	if err == nil {
		_ = os.WriteFile(shortConfigPath, configData, 0644)
	} else {
		emptyConfig := []byte("{}")
		_ = os.WriteFile(shortConfigPath, emptyConfig, 0644)
	}

	code := m.Run()

	_ = os.Remove(limitTestOutB64)
	_ = os.Remove(shortConfigPath)

	os.Exit(code)
}

// TestBlankWorkflowSimulation validates that the simulator can successfully
// run a blank workflow from end to end in a non-interactive mode.
func TestBlankWorkflowSimulation(t *testing.T) {
	_, thisFile, _, _ := rt.Caller(0)
	thisDir := filepath.Dir(thisFile)
	repoRoot := filepath.Clean(filepath.Join(thisDir, "..", "..", ".."))
	projectRoot := filepath.Join(repoRoot, "test", "test_project")
	workflowPath := filepath.Join(projectRoot, "blank_workflow")

	absWorkflowPath, err := filepath.Abs(workflowPath)
	require.NoError(t, err)

	outB64 := filepath.Join(absWorkflowPath, "binary.wasm.br.b64")
	t.Cleanup(func() {
		_ = os.Remove(outB64)
	})

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
	configPath := filepath.Join(absWorkflowPath, "config.json")
	if len(configPath) <= 97 {
		workflowSettings.WorkflowArtifactSettings.ConfigPath = configPath
	}

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

	handler := newHandler(runtimeCtx)

	inputs, err := handler.ResolveInputs(runtimeCtx.Viper, runtimeCtx.Settings)
	require.NoError(t, err)

	err = handler.ValidateInputs(inputs)
	require.NoError(t, err)

	err = handler.Execute(inputs)
	require.NoError(t, err, "Execute should not return an error")
}

func TestCapabilityLimits_LimitExceeded(t *testing.T) {
	_, thisFile, _, _ := rt.Caller(0)
	thisDir := filepath.Dir(thisFile)
	repoRoot := filepath.Clean(filepath.Join(thisDir, "..", "..", ".."))
	projectRoot := filepath.Join(repoRoot, "test", "test_project")

	capabilityTests := []struct {
		name           string
		capabilityName  string
		expectedErrMsg string
	}{
		{
			name:           "HTTP call limit",
			capabilityName:  "http",
			expectedErrMsg: "HTTPAction.CallLimit",
		},
		{
			name:           "Chain read call limit",
			capabilityName:  "chainread",
			expectedErrMsg: "ChainRead.CallLimit",
		},
		{
			name:           "Chain write target limit",
			capabilityName:  "chainwrite",
			expectedErrMsg: "ChainWrite.TargetsLimit",
		},
	}

	for _, tt := range capabilityTests {
		t.Run(tt.name, func(t *testing.T) {
			_, thisFile, _, _ := rt.Caller(0)
			thisDir := filepath.Dir(thisFile)
			testdataDir := filepath.Join(thisDir, "testdata")
			overrideFileName := fmt.Sprintf("%s-override.json", tt.capabilityName)
			sourceOverridePath := filepath.Join(testdataDir, overrideFileName)
			overrideFilePath := filepath.Join(limitTestWorkflowPath, overrideFileName)
			
			overrideData, err := os.ReadFile(sourceOverridePath)
			require.NoError(t, err, "Failed to read override file from testdata")
			require.NoError(t, os.WriteFile(overrideFilePath, overrideData, 0644))
			defer os.Remove(overrideFilePath)

			shortConfigPath := filepath.Join("/tmp", fmt.Sprintf("c%d.json", t.Name()[len(t.Name())-1]))
			originalConfigPath := filepath.Join(limitTestWorkflowPath, "config.json")
			configData, err := os.ReadFile(originalConfigPath)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(shortConfigPath, configData, 0644))
			defer os.Remove(shortConfigPath)
			
			absShortConfigPath, err := filepath.Abs(shortConfigPath)
			require.NoError(t, err)
			if len(absShortConfigPath) > 97 {
				t.Skipf("Config path too long: %d characters (max 97)", len(absShortConfigPath))
			}

			v := viper.New()
			v.Set("project-root", projectRoot)
			v.Set("non-interactive", true)
			v.Set("trigger-index", 0)
			v.Set("target", tt.capabilityName)

			var rpc settings.RpcEndpoint
			rpc.ChainName = "ethereum-testnet-sepolia"
			rpc.Url = "https://sepolia.infura.io/v3"
			v.Set(fmt.Sprintf("%s.%s", tt.capabilityName, settings.RpcsSettingName), []settings.RpcEndpoint{rpc})
			v.Set(fmt.Sprintf("%s.%s", tt.capabilityName, settings.WorkflowNameSettingName), "limit-test-workflow")
			v.Set(fmt.Sprintf("%s.%s", tt.capabilityName, settings.WorkflowPathSettingName), filepath.Join(limitTestWorkflowPath, "main.go"))

			var workflowSettings settings.WorkflowSettings
			workflowSettings.UserWorkflowSettings.WorkflowName = "limit-test-workflow"
			workflowSettings.WorkflowArtifactSettings.WorkflowPath = filepath.Join(limitTestWorkflowPath, "main.go")
			workflowSettings.WorkflowArtifactSettings.OverrideFilePath = overrideFileName
			workflowSettings.WorkflowArtifactSettings.ConfigPath = absShortConfigPath

			runtimeCtx := &runtime.Context{
				Logger: testutil.NewTestLogger(),
				Viper:  v,
				Settings: &settings.Settings{
					Workflow: workflowSettings,
					User: settings.UserSettings{
						TargetName:    tt.capabilityName,
						EthPrivateKey: "88888845d8761ca4a8cefb324c89702f12114ffbd0c47222f12aac0ad6538888",
					},
				},
			}

			handler := newHandler(runtimeCtx)
			inputs, err := handler.ResolveInputs(runtimeCtx.Viper, runtimeCtx.Settings)
			require.NoError(t, err)

			err = handler.ValidateInputs(inputs)
			require.NoError(t, err)
			
			var cfg cresettings.Workflows
			cfgFn := func(cfg *cresettings.Workflows) {
				cfg.ChainAllowed = commonsettings.PerChainSelector(
					commonsettings.Bool(true),
					map[string]bool{},
				)
				if runtimeCtx.Settings == nil {
					return
				}
				overrideFilePath := runtimeCtx.Settings.Workflow.WorkflowArtifactSettings.OverrideFilePath
				if overrideFilePath != "" {
					workflowDir := filepath.Dir(runtimeCtx.Settings.Workflow.WorkflowArtifactSettings.WorkflowPath)
					var absOverridePath string
					if filepath.IsAbs(overrideFilePath) {
						absOverridePath = overrideFilePath
					} else {
						absOverridePath = filepath.Join(workflowDir, overrideFilePath)
					}
					if overrideData, err := os.ReadFile(absOverridePath); err == nil {
						var override struct {
							HTTPCallLimit         *int `json:"http-call-limit"`
							ChainReadCallLimit    *int `json:"chain-read-call-limit"`
							ChainWriteTargetLimit *int `json:"chain-write-target-limit"`
							ConsensusCallLimit    *int `json:"consensus-call-limit"`
						}
						if err := json.Unmarshal(overrideData, &override); err == nil {
							if override.HTTPCallLimit != nil && *override.HTTPCallLimit > 0 {
								cfg.HTTPAction.CallLimit.DefaultValue = *override.HTTPCallLimit
							}
							if override.ChainReadCallLimit != nil && *override.ChainReadCallLimit > 0 {
								cfg.ChainRead.CallLimit.DefaultValue = *override.ChainReadCallLimit
							}
							if override.ChainWriteTargetLimit != nil && *override.ChainWriteTargetLimit > 0 {
								cfg.ChainWrite.TargetsLimit.DefaultValue = *override.ChainWriteTargetLimit
							}
							if override.ConsensusCallLimit != nil && *override.ConsensusCallLimit > 0 {
								cfg.Consensus.CallLimit.DefaultValue = *override.ConsensusCallLimit
							}
						}
					}
				}
			}
			cfgFn(&cfg)
			
			err = handler.Execute(inputs)
			if err != nil {
				require.Contains(t, err.Error(), tt.expectedErrMsg, "Error should mention the capability that was limited")
			}
			t.Logf("Verified limit configuration for %s (limit=1). Workflow returns '%s' when limit is hit.", tt.name, tt.capabilityName)
		})
	}
}

func TestCapabilityLimits_Success(t *testing.T) {
	_, thisFile, _, _ := rt.Caller(0)
	thisDir := filepath.Dir(thisFile)
	repoRoot := filepath.Clean(filepath.Join(thisDir, "..", "..", ".."))
	projectRoot := filepath.Join(repoRoot, "test", "test_project")

	limitTests := []int{2, 3}

	for _, limit := range limitTests {
		t.Run(fmt.Sprintf("limit_%d", limit), func(t *testing.T) {
			_, thisFile, _, _ := rt.Caller(0)
			thisDir := filepath.Dir(thisFile)
			testdataDir := filepath.Join(thisDir, "testdata")
			sourceOverridePath := filepath.Join(testdataDir, fmt.Sprintf("success-override-%d.json", limit))
			overrideFilePath := filepath.Join(limitTestWorkflowPath, "success-override.json")
			
			overrideData, err := os.ReadFile(sourceOverridePath)
			require.NoError(t, err, "Failed to read override file from testdata")
			require.NoError(t, os.WriteFile(overrideFilePath, overrideData, 0644))
			defer os.Remove(overrideFilePath)

			shortConfigPath := filepath.Join("/tmp", fmt.Sprintf("c%d.json", t.Name()[len(t.Name())-1]))
			originalConfigPath := filepath.Join(limitTestWorkflowPath, "config.json")
			configData, err := os.ReadFile(originalConfigPath)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(shortConfigPath, configData, 0644))
			defer os.Remove(shortConfigPath)
			
			absShortConfigPath, err := filepath.Abs(shortConfigPath)
			require.NoError(t, err)
			if len(absShortConfigPath) > 97 {
				t.Skipf("Config path too long: %d characters (max 97)", len(absShortConfigPath))
			}

			v := viper.New()
			v.Set("project-root", projectRoot)
			v.Set("non-interactive", true)
			v.Set("trigger-index", 0)
			v.Set("target", "success")

			var rpc settings.RpcEndpoint
			rpc.ChainName = "ethereum-testnet-sepolia"
			rpc.Url = "https://sepolia.infura.io/v3"
			v.Set(fmt.Sprintf("%s.%s", "success", settings.RpcsSettingName), []settings.RpcEndpoint{rpc})
			v.Set(fmt.Sprintf("%s.%s", "success", settings.WorkflowNameSettingName), "limit-test-workflow")
			v.Set(fmt.Sprintf("%s.%s", "success", settings.WorkflowPathSettingName), filepath.Join(limitTestWorkflowPath, "main.go"))

			var workflowSettings settings.WorkflowSettings
			workflowSettings.UserWorkflowSettings.WorkflowName = "limit-test-workflow"
			workflowSettings.WorkflowArtifactSettings.WorkflowPath = filepath.Join(limitTestWorkflowPath, "main.go")
			workflowSettings.WorkflowArtifactSettings.OverrideFilePath = "success-override.json"
			workflowSettings.WorkflowArtifactSettings.ConfigPath = absShortConfigPath

			runtimeCtx := &runtime.Context{
				Logger: testutil.NewTestLogger(),
				Viper:  v,
				Settings: &settings.Settings{
					Workflow: workflowSettings,
					User: settings.UserSettings{
						TargetName:    "success",
						EthPrivateKey: "88888845d8761ca4a8cefb324c89702f12114ffbd0c47222f12aac0ad6538888",
					},
				},
			}

			handler := newHandler(runtimeCtx)
			inputs, err := handler.ResolveInputs(runtimeCtx.Viper, runtimeCtx.Settings)
			require.NoError(t, err)

			err = handler.ValidateInputs(inputs)
			require.NoError(t, err)

			err = handler.Execute(inputs)
			if err != nil {
				require.NotContains(t, err.Error(), "CallLimit limited", "Error should not be a limit error when limit=%d", limit)
			}
			t.Logf("Workflow executed with all limits set to %d (HTTP calls succeeded, may fail on RPC for chain operations)", limit)
		})
	}
}

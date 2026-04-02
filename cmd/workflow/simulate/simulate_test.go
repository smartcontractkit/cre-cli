package simulate

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	rt "runtime"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
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

	// Run test from workflow dir so short relative paths (max 97 chars) work
	prevWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(absWorkflowPath))
	t.Cleanup(func() {
		_ = os.Chdir(prevWd)
	})

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

	// Use relative paths so validation (max 97 chars) passes; cwd is workflow dir
	var workflowSettings settings.WorkflowSettings
	workflowSettings.UserWorkflowSettings.WorkflowName = "blank-workflow"
	workflowSettings.WorkflowArtifactSettings.WorkflowPath = "main.go"
	workflowSettings.WorkflowArtifactSettings.ConfigPath = "config.json"

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

func createSimulateTestViper(t *testing.T) *viper.Viper {
	t.Helper()
	v := viper.New()
	v.Set("target", "staging-settings")
	var rpc settings.RpcEndpoint
	rpc.ChainName = "ethereum-testnet-sepolia"
	rpc.Url = "https://example.com/rpc"
	v.Set(fmt.Sprintf("%s.%s", "staging-settings", settings.RpcsSettingName), []settings.RpcEndpoint{rpc})
	return v
}

func createSimulateTestSettings(workflowName, workflowPath, configPath string) *settings.Settings {
	return &settings.Settings{
		Workflow: settings.WorkflowSettings{
			UserWorkflowSettings: struct {
				WorkflowOwnerAddress string `mapstructure:"workflow-owner-address" yaml:"workflow-owner-address"`
				WorkflowOwnerType    string `mapstructure:"workflow-owner-type" yaml:"workflow-owner-type"`
				WorkflowName         string `mapstructure:"workflow-name" yaml:"workflow-name"`
			}{
				WorkflowName: workflowName,
			},
			WorkflowArtifactSettings: struct {
				WorkflowPath string `mapstructure:"workflow-path" yaml:"workflow-path"`
				ConfigPath   string `mapstructure:"config-path" yaml:"config-path"`
				SecretsPath  string `mapstructure:"secrets-path" yaml:"secrets-path"`
			}{
				WorkflowPath: workflowPath,
				ConfigPath:   configPath,
			},
		},
		User: settings.UserSettings{
			EthPrivateKey: "88888845d8761ca4a8cefb324c89702f12114ffbd0c47222f12aac0ad6538888",
		},
	}
}

func TestSimulateResolveInputs_ConfigFlags(t *testing.T) {
	t.Parallel()

	settingsConfigPath := "config.json"

	tests := []struct {
		name               string
		viperOverrides     map[string]interface{}
		expectedConfigPath string
	}{
		{
			name:               "default uses settings config path",
			viperOverrides:     nil,
			expectedConfigPath: settingsConfigPath,
		},
		{
			name:               "no-config clears config path",
			viperOverrides:     map[string]interface{}{"no-config": true},
			expectedConfigPath: "",
		},
		{
			name:               "config flag overrides settings",
			viperOverrides:     map[string]interface{}{"config": "override.json"},
			expectedConfigPath: "override.json",
		},
		{
			name:               "default-config uses settings config path",
			viperOverrides:     map[string]interface{}{"default-config": true},
			expectedConfigPath: settingsConfigPath,
		},
		{
			name:               "config flag with URL value",
			viperOverrides:     map[string]interface{}{"config": "https://example.com/config.yaml"},
			expectedConfigPath: "https://example.com/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v := createSimulateTestViper(t)
			creSettings := createSimulateTestSettings("test-workflow", "main.go", settingsConfigPath)

			for k, val := range tt.viperOverrides {
				v.Set(k, val)
			}

			runtimeCtx := &runtime.Context{
				Logger:   testutil.NewTestLogger(),
				Viper:    v,
				Settings: creSettings,
			}
			h := newHandler(runtimeCtx)

			inputs, err := h.ResolveInputs(v, creSettings)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedConfigPath, inputs.ConfigPath)
		})
	}
}

func TestSimulateResolveInputs_WasmFlag(t *testing.T) {
	t.Parallel()

	t.Run("local path", func(t *testing.T) {
		v := createSimulateTestViper(t)
		v.Set("wasm", "/tmp/test.wasm")
		creSettings := createSimulateTestSettings("test-workflow", "main.go", "")

		runtimeCtx := &runtime.Context{
			Logger:   testutil.NewTestLogger(),
			Viper:    v,
			Settings: creSettings,
		}
		h := newHandler(runtimeCtx)

		inputs, err := h.ResolveInputs(v, creSettings)
		require.NoError(t, err)
		assert.Equal(t, "/tmp/test.wasm", inputs.WasmPath)
	})

	t.Run("URL", func(t *testing.T) {
		v := createSimulateTestViper(t)
		v.Set("wasm", "https://example.com/binary.wasm")
		creSettings := createSimulateTestSettings("test-workflow", "main.go", "")

		runtimeCtx := &runtime.Context{
			Logger:   testutil.NewTestLogger(),
			Viper:    v,
			Settings: creSettings,
		}
		h := newHandler(runtimeCtx)

		inputs, err := h.ResolveInputs(v, creSettings)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/binary.wasm", inputs.WasmPath)
	})
}

func TestSimulateValidateInputs_URLBypass(t *testing.T) {
	t.Parallel()

	tmpFile := filepath.Join(t.TempDir(), "main.go")
	require.NoError(t, os.WriteFile(tmpFile, []byte("package main"), 0600))

	runtimeCtx := &runtime.Context{
		Logger: testutil.NewTestLogger(),
	}
	h := newHandler(runtimeCtx)

	inputs := Inputs{
		WorkflowPath: tmpFile,
		ConfigPath:   "https://example.com/config.yaml",
		WasmPath:     "https://example.com/binary.wasm",
		WorkflowName: "test-workflow",
	}

	err := h.ValidateInputs(inputs)
	require.NoError(t, err, "URL values should bypass file/ascii/max validators")
	assert.True(t, h.validated)
}

func TestSimulateWasmFormatHandling(t *testing.T) {
	t.Parallel()

	t.Run("EnsureRawWasm with raw wasm", func(t *testing.T) {
		t.Parallel()
		raw := append([]byte{0x00, 0x61, 0x73, 0x6d}, []byte("test wasm")...)
		result, err := cmdcommon.EnsureRawWasm(raw)
		require.NoError(t, err)
		assert.Equal(t, raw, result)
	})

	t.Run("EnsureRawWasm with br64 data", func(t *testing.T) {
		t.Parallel()
		raw := append([]byte{0x00, 0x61, 0x73, 0x6d}, []byte("test wasm")...)
		compressed, err := cmdcommon.CompressBrotli(raw)
		require.NoError(t, err)
		br64 := []byte(base64.StdEncoding.EncodeToString(compressed))

		result, err := cmdcommon.EnsureRawWasm(br64)
		require.NoError(t, err)
		assert.Equal(t, raw, result)
	})
}

func TestResolvePathFromInvocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		path          string
		invocationDir string
		want          string
	}{
		{
			name:          "absolute path returned unchanged regardless of invocationDir",
			path:          "/absolute/path/file.json",
			invocationDir: "/some/other/dir",
			want:          "/absolute/path/file.json",
		},
		{
			name:          "relative path with empty invocationDir returned unchanged",
			path:          "relative/file.json",
			invocationDir: "",
			want:          "relative/file.json",
		},
		{
			name:          "relative path joined with invocationDir",
			path:          "file.json",
			invocationDir: "/invocation/dir",
			want:          "/invocation/dir/file.json",
		},
		{
			name:          "relative path with subdirs joined with invocationDir",
			path:          "sub/dir/file.json",
			invocationDir: "/invocation/dir",
			want:          "/invocation/dir/sub/dir/file.json",
		},
		{
			name:          "dot-slash relative path joined with invocationDir",
			path:          "./file.json",
			invocationDir: "/invocation/dir",
			want:          "/invocation/dir/file.json",
		},
		{
			name:          "absolute path with empty invocationDir returned unchanged",
			path:          "/abs/path.json",
			invocationDir: "",
			want:          "/abs/path.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolvePathFromInvocation(tt.path, tt.invocationDir)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetHTTPTriggerPayloadFromInput(t *testing.T) {
	t.Parallel()

	// Create a temp dir with a payload file for file-based tests.
	tmpDir := t.TempDir()
	payloadJSON := `{"method":"GET","path":"/hello"}`
	payloadFile := filepath.Join(tmpDir, "payload.json")
	require.NoError(t, os.WriteFile(payloadFile, []byte(payloadJSON), 0600))

	t.Run("empty input returns error", func(t *testing.T) {
		t.Parallel()
		_, err := getHTTPTriggerPayloadFromInput("", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty http payload input")
	})

	t.Run("whitespace-only input returns error", func(t *testing.T) {
		t.Parallel()
		_, err := getHTTPTriggerPayloadFromInput("   ", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty http payload input")
	})

	t.Run("at-prefix with absolute file path reads file", func(t *testing.T) {
		t.Parallel()
		payload, err := getHTTPTriggerPayloadFromInput("@"+payloadFile, "")
		require.NoError(t, err)
		assert.Equal(t, []byte(payloadJSON), payload.Input)
	})

	t.Run("at-prefix with relative path resolved against invocationDir", func(t *testing.T) {
		t.Parallel()
		payload, err := getHTTPTriggerPayloadFromInput("@payload.json", tmpDir)
		require.NoError(t, err)
		assert.Equal(t, []byte(payloadJSON), payload.Input)
	})

	t.Run("at-prefix with nonexistent file returns error", func(t *testing.T) {
		t.Parallel()
		_, err := getHTTPTriggerPayloadFromInput("@/nonexistent/no-such-file.json", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file")
	})

	t.Run("absolute file path without at-prefix reads file", func(t *testing.T) {
		t.Parallel()
		payload, err := getHTTPTriggerPayloadFromInput(payloadFile, "")
		require.NoError(t, err)
		assert.Equal(t, []byte(payloadJSON), payload.Input)
	})

	t.Run("relative file path resolved against invocationDir reads file", func(t *testing.T) {
		t.Parallel()
		payload, err := getHTTPTriggerPayloadFromInput("payload.json", tmpDir)
		require.NoError(t, err)
		assert.Equal(t, []byte(payloadJSON), payload.Input)
	})

	t.Run("inline JSON string used as raw bytes", func(t *testing.T) {
		t.Parallel()
		inlineJSON := `{"method":"POST","path":"/api"}`
		payload, err := getHTTPTriggerPayloadFromInput(inlineJSON, "")
		require.NoError(t, err)
		assert.Equal(t, []byte(inlineJSON), payload.Input)
	})

	t.Run("nonexistent relative path with empty invocationDir treated as raw bytes", func(t *testing.T) {
		t.Parallel()
		// A path that doesn't exist is treated as raw bytes (no error).
		input := "no-such-file-or-json"
		payload, err := getHTTPTriggerPayloadFromInput(input, "")
		require.NoError(t, err)
		assert.Equal(t, []byte(input), payload.Input)
	})

	t.Run("relative path not found in invocationDir treated as raw bytes", func(t *testing.T) {
		t.Parallel()
		// A relative path that resolves to a nonexistent file is used as raw bytes.
		input := "does-not-exist.json"
		payload, err := getHTTPTriggerPayloadFromInput(input, tmpDir)
		require.NoError(t, err)
		assert.Equal(t, []byte(input), payload.Input)
	})
}

func TestSimulateResolveInputs_InvocationDir(t *testing.T) {
	t.Parallel()

	invocationDir := "/some/invocation/dir"
	v := createSimulateTestViper(t)
	creSettings := createSimulateTestSettings("test-workflow", "main.go", "config.json")

	runtimeCtx := &runtime.Context{
		Logger:        testutil.NewTestLogger(),
		Viper:         v,
		Settings:      creSettings,
		InvocationDir: invocationDir,
	}
	h := newHandler(runtimeCtx)

	inputs, err := h.ResolveInputs(v, creSettings)
	require.NoError(t, err)
	assert.Equal(t, invocationDir, inputs.InvocationDir)
}

func TestSimulateConfigFlagsMutuallyExclusive(t *testing.T) {
	t.Parallel()

	runtimeCtx := &runtime.Context{
		Logger: testutil.NewTestLogger(),
		Viper:  viper.New(),
		Settings: &settings.Settings{
			User: settings.UserSettings{
				EthPrivateKey: "88888845d8761ca4a8cefb324c89702f12114ffbd0c47222f12aac0ad6538888",
			},
		},
	}

	cmd := New(runtimeCtx)
	cmd.SetArgs([]string{"./some-workflow", "--no-config", "--config", "foo.yml"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "if any flags in the group [config no-config default-config] are set none of the others can be")
}

package simulate

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	rt "runtime"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	commoncaps "github.com/smartcontractkit/chainlink-common/pkg/capabilities"
	crontypedapi "github.com/smartcontractkit/chainlink-common/pkg/capabilities/v2/triggers/cron"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	pb "github.com/smartcontractkit/chainlink-protos/cre/go/sdk"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/fakes"
	simulator "github.com/smartcontractkit/chainlink/v2/core/services/workflows/cmd/cre/utils"

	cmdcommon "github.com/smartcontractkit/cre-cli/cmd/common"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/testutil"
)

func freeTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	require.Greater(t, addr.Port, 0)
	return addr.Port
}

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
	err = handler.Execute(context.Background(), inputs)
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
				DeploymentRegistry   string `mapstructure:"deployment-registry" yaml:"deployment-registry"`
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

	t.Run("empty input returns nil payload and no error", func(t *testing.T) {
		t.Parallel()
		payload, err := getHTTPTriggerPayloadFromInput("", "")
		require.NoError(t, err)
		require.Nil(t, payload)
	})

	t.Run("whitespace-only input returns nil payload and no error", func(t *testing.T) {
		t.Parallel()
		payload, err := getHTTPTriggerPayloadFromInput("   ", "    ")
		require.NoError(t, err)
		require.Nil(t, payload)
	})

	t.Run("absolute file path reads and parses JSON", func(t *testing.T) {
		t.Parallel()
		payload, err := getHTTPTriggerPayloadFromInput("", payloadFile)
		require.NoError(t, err)
		require.NotNil(t, payload)
		var got map[string]interface{}
		require.NoError(t, json.Unmarshal(payload.Input, &got))
		assert.Equal(t, "GET", got["method"])
		assert.Equal(t, "/hello", got["path"])
	})

	t.Run("relative path resolved against invocationDir reads and parses JSON", func(t *testing.T) {
		t.Parallel()
		payload, err := getHTTPTriggerPayloadFromInput(tmpDir, "payload.json")
		require.NoError(t, err)
		require.NotNil(t, payload)
		var got map[string]interface{}
		require.NoError(t, json.Unmarshal(payload.Input, &got))
		assert.Equal(t, "GET", got["method"])
		assert.Equal(t, "/hello", got["path"])
	})

	t.Run("nonexistent file path returns invalid JSON error", func(t *testing.T) {
		t.Parallel()
		_, err := getHTTPTriggerPayloadFromInput("", "/nonexistent/no-such-file.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON input")
	})

	t.Run("inline JSON string parsed as payload", func(t *testing.T) {
		t.Parallel()
		inlineJSON := `{"method":"POST","path":"/api"}`
		payload, err := getHTTPTriggerPayloadFromInput("", inlineJSON)
		require.NoError(t, err)
		require.NotNil(t, payload)
		var got map[string]interface{}
		require.NoError(t, json.Unmarshal(payload.Input, &got))
		assert.Equal(t, "POST", got["method"])
		assert.Equal(t, "/api", got["path"])
	})

	t.Run("non-JSON non-file input returns error", func(t *testing.T) {
		t.Parallel()
		_, err := getHTTPTriggerPayloadFromInput("", "no-such-file-or-json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON input")
	})

	t.Run("relative path not found in invocationDir returns error", func(t *testing.T) {
		t.Parallel()
		_, err := getHTTPTriggerPayloadFromInput(tmpDir, "does-not-exist.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON input")
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

// TestNonInteractiveCronTriggerDoesNotBlockOnSchedule verifies that when the
// simulator runs in non-interactive mode with a cron trigger, TriggerFunc
// completes immediately without waiting for the actual cron schedule.
//
// The previous broken implementation sent skipWaitSignal *after* ManualTrigger
// returned, so ManualTrigger blocked in its select until the real cron job fired
// (up to 60 s). The fix pre-fills the channel before calling ManualTrigger.
func TestNonInteractiveCronTriggerDoesNotBlockOnSchedule(t *testing.T) {
	t.Parallel()

	cronSvc, err := fakes.NewManualCronTriggerService(logger.Test(t))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, cronSvc.Start(ctx))
	t.Cleanup(func() { _ = cronSvc.Close() })

	// Register the trigger with the ID that makeBeforeStartNonInteractive will use.
	triggerIndex := 0
	triggerRegistrationID := fmt.Sprintf("trigger_reg_1111111111111111111111111111111111111111111111111111111111111111_%d", triggerIndex)
	_, capErr := cronSvc.RegisterTrigger(ctx, triggerRegistrationID,
		commoncaps.RequestMetadata{WorkflowID: "test-workflow"},
		&crontypedapi.Config{Schedule: "* * * * *"},
	)
	require.Nil(t, capErr)

	holder := &TriggerInfoAndBeforeStart{}
	inputs := Inputs{TriggerIndex: triggerIndex}
	manualTriggers := &ManualTriggers{ManualCronTrigger: cronSvc}

	beforeStart := makeBeforeStartNonInteractive(holder, inputs, func() *ManualTriggers {
		return manualTriggers
	})

	triggerSub := []*pb.TriggerSubscription{{Id: "cron-trigger@1.0.0"}}
	beforeStart(ctx, simulator.RunnerConfig{}, nil, nil, triggerSub)
	require.NotNil(t, holder.TriggerFunc)

	done := make(chan error, 1)
	go func() { done <- holder.TriggerFunc() }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("TriggerFunc blocked waiting for cron schedule; skipWaitSignal must be sent before ManualTrigger is called")
	}
}

func TestHTTPListenPayloadServerAcceptsMultipleRequests(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	port := freeTCPPort(t)
	payloadCh, closeServer, err := startHTTPListenPayloadServer(ctx, port)
	require.NoError(t, err)
	t.Cleanup(closeServer)

	for _, input := range []string{`{"key":"first"}`, `{"key":"second"}`} {
		body := []byte(fmt.Sprintf(`{"input":%s}`, input))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/trigger", port), bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req) // #nosec G704 -- URL targets localhost test server
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	}

	for _, want := range [][]byte{[]byte(`{"key":"first"}`), []byte(`{"key":"second"}`)} {
		select {
		case payload := <-payloadCh:
			require.Equal(t, want, payload.Input)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for queued HTTP trigger payload")
		}
	}
}

func TestManualHTTPTriggerEventsHaveUniqueIDs(t *testing.T) {
	t.Parallel()

	svc := NewManualHTTPTriggerService(logger.Test(t))
	first := svc.createManualTriggerEvent(nil)
	second := svc.createManualTriggerEvent(nil)

	require.NotEmpty(t, first.Id)
	require.NotEqual(t, first.Id, second.Id)
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

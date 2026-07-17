package telemetry

import (
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/redact"
)

func TestCollectMachineInfo(t *testing.T) {
	info := CollectMachineInfo()

	assert.Equal(t, runtime.GOOS, info.OsName)
	assert.Equal(t, runtime.GOARCH, info.Architecture)
	assert.NotEmpty(t, info.OsVersion)
}

func TestCollectCommandInfo(t *testing.T) {
	tests := []struct {
		name           string
		cmd            *cobra.Command
		expectedAction string
		expectedSub    string
	}{
		{
			name: "top level command",
			cmd: &cobra.Command{
				Use: "login",
			},
			expectedAction: "login",
			expectedSub:    "",
		},
		{
			name: "subcommand",
			cmd: func() *cobra.Command {
				parent := &cobra.Command{Use: "workflow"}
				child := &cobra.Command{Use: "deploy"}
				parent.AddCommand(child)
				return child
			}(),
			expectedAction: "workflow",
			expectedSub:    "deploy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := CollectCommandInfo(tt.cmd, []string{})
			assert.Equal(t, tt.expectedAction, info.Action)
			assert.Equal(t, tt.expectedSub, info.Subcommand)
		})
	}
}

func TestIsTelemetryDisabled(t *testing.T) {
	// Save original state
	originalValue, wasSet := os.LookupEnv(TelemetryDisabledEnvVar)
	defer func() {
		if wasSet {
			os.Setenv(TelemetryDisabledEnvVar, originalValue)
		} else {
			os.Unsetenv(TelemetryDisabledEnvVar)
		}
	}()

	// Test when not set (enabled)
	os.Unsetenv(TelemetryDisabledEnvVar)
	assert.False(t, isTelemetryDisabled())

	// Test when set to "true" (disabled)
	os.Setenv(TelemetryDisabledEnvVar, "true")
	assert.True(t, isTelemetryDisabled())

	// Test when set to "false" (enabled)
	os.Setenv(TelemetryDisabledEnvVar, "false")
	assert.False(t, isTelemetryDisabled())

	// Test when set to other values (enabled)
	os.Setenv(TelemetryDisabledEnvVar, "1")
	assert.False(t, isTelemetryDisabled())

	os.Setenv(TelemetryDisabledEnvVar, "")
	assert.False(t, isTelemetryDisabled())
}

func TestShouldExcludeCommand(t *testing.T) {
	tests := []struct {
		name          string
		cmdName       string
		shouldExclude bool
	}{
		{"version", "version", true},
		{"help", "help", true},
		{"bash completion", "bash", true},
		{"zsh completion", "zsh", true},
		{"fish completion", "fish", true},
		{"powershell completion", "powershell", true},
		{"completion", "completion", true},
		{"login", "login", false},
		{"workflow", "workflow", false},
		{"deploy", "deploy", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: tt.cmdName}
			assert.Equal(t, tt.shouldExclude, shouldExcludeCommand(cmd))
		})
	}
}

func TestBuildUserEvent(t *testing.T) {
	cmd := &cobra.Command{Use: "login"}
	exitCode := 0

	event := buildUserEvent(cmd, []string{}, exitCode, nil, nil)

	assert.NotEmpty(t, event.CliVersion)
	assert.Equal(t, exitCode, event.ExitCode)
	assert.Equal(t, "login", event.Command.Action)
	assert.Equal(t, runtime.GOOS, event.Machine.OsName)
	assert.Equal(t, runtime.GOARCH, event.Machine.Architecture)
}

func TestCollectCommandInfo_RedactsSensitiveFlags(t *testing.T) {
	parent := &cobra.Command{Use: "workflow"}
	cmd := &cobra.Command{Use: "simulate"}
	parent.AddCommand(cmd)

	cmd.Flags().String("env", "", "env file")
	cmd.Flags().String("http-payload", "", "payload")
	cmd.Flags().String("wasm", "", "wasm path")
	require.NoError(t, cmd.Flags().Set("env", "/home/user/.env"))
	require.NoError(t, cmd.Flags().Set("http-payload", `{"token":"secret"}`))
	require.NoError(t, cmd.Flags().Set("wasm", "https://cdn.example.com/wasm/secret"))

	info := CollectCommandInfo(cmd, []string{"./my-workflow"})

	assert.Equal(t, "workflow", info.Action)
	assert.Equal(t, "simulate", info.Subcommand)
	assert.Equal(t, []string{"./my-workflow"}, info.Args)

	flagValues := map[string]string{}
	for _, flag := range info.Flags {
		flagValues[flag.Key] = flag.Value
	}
	assert.Equal(t, redact.RedactedValue, flagValues["env"])
	assert.Equal(t, redact.RedactedValue, flagValues["http-payload"])
	assert.Equal(t, "https://cdn.example.com/wasm/***", flagValues["wasm"])
}

func TestCollectCommandInfo_RedactsSecretsArgs(t *testing.T) {
	parent := &cobra.Command{Use: "secrets"}
	cmd := &cobra.Command{Use: "create"}
	parent.AddCommand(cmd)

	info := CollectCommandInfo(cmd, []string{"/home/user/project/secrets.yaml"})
	assert.Equal(t, []string{"secrets.yaml"}, info.Args)
}

func TestBuildUserEvent_RedactsErrorMessage(t *testing.T) {
	cmd := &cobra.Command{Use: "login"}
	err := errors.New("auth failed: Bearer super-secret-token")

	event := buildUserEvent(cmd, []string{}, 1, nil, err)

	assert.Equal(t, "auth failed: Bearer "+redact.RedactedValue, event.ErrorMessage)
}

func TestGetOSVersion(t *testing.T) {
	version := getOSVersion()
	require.NotEmpty(t, version)

	// Should either get a real version or "unknown"
	// We can't test the exact value as it depends on the OS
	t.Logf("Detected OS version: %s", version)
}

func TestIsTelemetryDebugEnabled(t *testing.T) {
	// Save original state
	originalValue, wasSet := os.LookupEnv(TelemetryDebugEnvVar)
	defer func() {
		if wasSet {
			os.Setenv(TelemetryDebugEnvVar, originalValue)
		} else {
			os.Unsetenv(TelemetryDebugEnvVar)
		}
	}()

	// Test when not set (disabled by default)
	os.Unsetenv(TelemetryDebugEnvVar)
	assert.False(t, isTelemetryDebugEnabled())

	// Test when set to "true" (enabled)
	os.Setenv(TelemetryDebugEnvVar, "true")
	assert.True(t, isTelemetryDebugEnabled())

	// Test when set to "false" (disabled)
	os.Setenv(TelemetryDebugEnvVar, "false")
	assert.False(t, isTelemetryDebugEnabled())

	// Test when set to other values (disabled)
	os.Setenv(TelemetryDebugEnvVar, "1")
	assert.False(t, isTelemetryDebugEnabled())

	os.Setenv(TelemetryDebugEnvVar, "")
	assert.False(t, isTelemetryDebugEnabled())
}

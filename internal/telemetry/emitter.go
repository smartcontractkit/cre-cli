package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/version"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

const (
	// TelemetryDisabledEnvVar is the environment variable that disables telemetry when set
	TelemetryDisabledEnvVar = "CRE_TELEMETRY_DISABLED"

	// TelemetryDebugEnvVar is the environment variable that enables debug logging for telemetry
	TelemetryDebugEnvVar = "CRE_TELEMETRY_DEBUG"

	// Maximum time to wait for telemetry to complete
	maxTelemetryWait = 1 * time.Second
)

// EmitCommandEvent emits a user event for command execution. It has a max timeout maxTelemetryWait
// which the user needs to wait for
func EmitCommandEvent(cmd *cobra.Command, exitCode int, runtimeCtx *runtime.Context) {
	// Recover from any panics to prevent crashes
	defer func() {
		if r := recover(); r != nil && isTelemetryDebugEnabled() {
			debugLog("telemetry panic recovered: %v", r)
		}
	}()

	// Create context with timeout
	emitCtx, cancel := context.WithTimeout(context.Background(), maxTelemetryWait)
	defer cancel()

	// Check if telemetry is disabled
	if isTelemetryDisabled() {
		debugLog("telemetry disabled via environment variable")
		return
	}

	// Check if this command should be excluded
	if shouldExcludeCommand(cmd) {
		debugLog("command %s excluded from telemetry", cmd.Name())
		return
	}

	// Collect event data
	event := buildUserEvent(cmd, exitCode)
	debugLog("emitting telemetry event: action=%s, subcommand=%s, exitCode=%d",
		event.Command.Action, event.Command.Subcommand, event.ExitCode)

	// Send the event
	SendEvent(emitCtx, event, runtimeCtx.Credentials, runtimeCtx.EnvironmentSet, runtimeCtx.Logger)
}

// isTelemetryDisabled checks if telemetry is disabled via environment variable
func isTelemetryDisabled() bool {
	value := os.Getenv(TelemetryDisabledEnvVar)
	return value == "true"
}

// isTelemetryDebugEnabled checks if telemetry debug logging is enabled
func isTelemetryDebugEnabled() bool {
	value := os.Getenv(TelemetryDebugEnvVar)
	return value == "true"
}

// debugLog logs a message if telemetry debug is enabled
func debugLog(format string, args ...interface{}) {
	if isTelemetryDebugEnabled() {
		fmt.Fprintf(os.Stderr, "[TELEMETRY DEBUG] "+format+"\n", args...)
	}
}

// shouldExcludeCommand determines if a command should not emit telemetry events
func shouldExcludeCommand(cmd *cobra.Command) bool {
	if cmd == nil {
		return true
	}

	excludedCommands := map[string]bool{
		"version":    true,
		"help":       true,
		"bash":       true,
		"zsh":        true,
		"fish":       true,
		"powershell": true,
		"completion": true,
	}

	cmdName := cmd.Name()
	return excludedCommands[cmdName]
}

// buildUserEvent constructs the user event payload
func buildUserEvent(cmd *cobra.Command, exitCode int) UserEventInput {
	return UserEventInput{
		CliVersion: version.Version,
		ExitCode:   exitCode,
		Command:    CollectCommandInfo(cmd),
		Machine:    CollectMachineInfo(),
	}
}

package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/cmd/version"
)

const (
	// TelemetryDisabledEnvVar is the environment variable that disables telemetry when set
	TelemetryDisabledEnvVar = "CRE_TELEMETRY_DISABLED"

	// TelemetryDebugEnvVar is the environment variable that enables debug logging for telemetry
	TelemetryDebugEnvVar = "CRE_TELEMETRY_DEBUG"

	// Maximum time to wait for telemetry to complete
	maxTelemetryWait = 10 * time.Second
)

var (
	debugLogFile *os.File
	logOnce      sync.Once
	logMutex     sync.Mutex
)

// IsTelemetryDebugEnabled checks if telemetry debug logging is enabled
func IsTelemetryDebugEnabled() bool {
	value := os.Getenv(TelemetryDebugEnvVar)
	return value == "true"
}

// GetLogfilePath returns the path to the debug log file
func GetLogfilePath() string {
	return filepath.Join(os.TempDir(), "cre_telemetry.log")
}

// DebugLog logs a message if telemetry debug is enabled
func DebugLog(format string, args ...interface{}) {
	if !IsTelemetryDebugEnabled() {
		return
	}

	// Initialize the log file once per process
	logOnce.Do(func() {
		var err error
		logPath := GetLogfilePath() // Use helper

		// Use os.O_CREATE|os.O_APPEND|os.O_WRONLY to append to the file
		debugLogFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			// Fallback to stderr if file open fails
			fmt.Fprintf(os.Stderr, "[TELEMETRY DEBUG] FAILED TO OPEN LOG FILE: %v\n", err)
			debugLogFile = os.Stderr
		} else {
			// Print to stderr *once* to let the user know where to look
			fmt.Fprintf(os.Stderr, "[TELEMETRY DEBUG] Logging to %s\n", logPath)
		}
	})

	// Write to the log file (or stderr fallback)
	if debugLogFile != nil {
		logMutex.Lock()
		defer logMutex.Unlock()

		// Create the log message with a timestamp
		allArgs := append([]interface{}{time.Now().Format(time.RFC3339Nano)}, args...)
		msg := fmt.Sprintf("[TELEMETRY DEBUG %s] "+format+"\n", allArgs...)

		debugLogFile.WriteString(msg)
	}
}

// ShouldExcludeCommand determines if a command should not emit telemetry events
func ShouldExcludeCommand(cmd *cobra.Command) bool {
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

// BuildUserEvent constructs the user event payload
func BuildUserEvent(cmd *cobra.Command, exitCode int) UserEventInput {
	return UserEventInput{
		CliVersion: version.Version,
		ExitCode:   exitCode,
		Command:    CollectCommandInfo(cmd),
		Machine:    CollectMachineInfo(),
	}
}

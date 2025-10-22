package simulate

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
)

// LogLevel represents the level of a simulation log
type LogLevel string

const (
	LogLevelDebug   LogLevel = "DEBUG"
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarning LogLevel = "WARNING"
	LogLevelError   LogLevel = "ERROR"
)

// Color instances for consistent styling
var (
	ColorBlue       = color.New(color.FgBlue)
	ColorBrightCyan = color.New(color.FgCyan, color.Bold)
	ColorYellow     = color.New(color.FgYellow)
	ColorRed        = color.New(color.FgRed)
	ColorGreen      = color.New(color.FgGreen)
	ColorMagenta    = color.New(color.FgMagenta)
)

// SimulationLogger provides an easy interface for formatted simulation logs
type SimulationLogger struct {
	verbosity bool
}

// NewSimulationLogger creates a new simulation logger with verbosity control
func NewSimulationLogger(verbosity bool) *SimulationLogger {
	// Smart color detection for end users
	enableColors := shouldEnableColors()
	color.NoColor = !enableColors
	return &SimulationLogger{verbosity: verbosity}
}

// Info logs an info level message with colored formatting
func (s *SimulationLogger) Info(message string, fields ...interface{}) {
	s.formatSimulationLog(LogLevelInfo, message, fields...)
}

// Debug logs a debug level message with colored formatting (only if verbosity is enabled)
func (s *SimulationLogger) Debug(message string, fields ...interface{}) {
	if s.verbosity {
		s.formatSimulationLog(LogLevelDebug, message, fields...)
	}
}

// Warn logs a warning level message with colored formatting
func (s *SimulationLogger) Warn(message string, fields ...interface{}) {
	s.formatSimulationLog(LogLevelWarning, message, fields...)
}

// Error logs an error level message with colored formatting
func (s *SimulationLogger) Error(message string, fields ...interface{}) {
	s.formatSimulationLog(LogLevelError, message, fields...)
}

// formatSimulationLog formats simulation logs with consistent styling and different levels
func (s *SimulationLogger) formatSimulationLog(level LogLevel, message string, fields ...interface{}) {
	// Get current timestamp
	timestamp := time.Now().Format("2006-01-02T15:04:05Z")

	// Format fields if provided
	formattedMessage := message
	if len(fields) > 0 {
		// Convert fields to key=value pairs
		var fieldPairs []string
		for i := 0; i < len(fields); i += 2 {
			if i+1 < len(fields) {
				fieldPairs = append(fieldPairs, fmt.Sprintf("%v=%v", fields[i], fields[i+1]))
			}
		}
		if len(fieldPairs) > 0 {
			formattedMessage = message + " " + strings.Join(fieldPairs, " ")
		}
	}

	// Get color for the log level
	var levelColor *color.Color
	switch level {
	case LogLevelDebug:
		levelColor = ColorBlue
	case LogLevelInfo:
		levelColor = ColorBrightCyan
	case LogLevelWarning:
		levelColor = ColorYellow
	case LogLevelError:
		levelColor = ColorRed
	default:
		levelColor = ColorBrightCyan
	}

	// Format with timestamp and level-specific color
	ColorBlue.Printf("%s ", timestamp)
	levelColor.Printf("[SIMULATION]")
	fmt.Printf(" %s\n", formattedMessage)
}

// PrintTimestampedLog prints a log with timestamp and colored prefix
func (s *SimulationLogger) PrintTimestampedLog(timestamp, prefix, message string, prefixColor *color.Color) {
	ColorBlue.Printf("%s ", timestamp)
	prefixColor.Printf("[%s]", prefix)
	fmt.Printf(" %s\n", message)
}

// PrintTimestampedLogWithStatus prints a log with timestamp, prefix, and colored status
func (s *SimulationLogger) PrintTimestampedLogWithStatus(timestamp, prefix, message, status string) {
	ColorBlue.Printf("%s ", timestamp)
	ColorMagenta.Printf("[%s]", prefix)
	fmt.Printf(" %s", message)
	statusColor := GetColor(status)
	statusColor.Printf("%s\n", status)
}

// PrintStepLog prints a capability step log with timestamp and colored status
func (s *SimulationLogger) PrintStepLog(timestamp, component, stepRef, capability, status string) {
	ColorBlue.Printf("%s ", timestamp)
	ColorBrightCyan.Printf("[%s]", component)
	fmt.Printf("       step[%s]   Capability: %s - ", stepRef, capability)
	statusColor := GetColor(status)
	statusColor.Printf("%s\n", status)
}

// PrintWorkflowMetadata prints workflow metadata with proper indentation
func (s *SimulationLogger) PrintWorkflowMetadata(metadata interface{}) {
	if metadata == nil {
		return
	}

	// Use reflection to print metadata fields
	// This is a generic implementation that works with any struct
	v := reflect.ValueOf(metadata)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// Skip unexported fields
		if !value.CanInterface() {
			continue
		}

		// Get field name and value
		fieldName := field.Name
		fieldValue := value.Interface()

		// Skip empty values
		if isEmptyValue(fieldValue) {
			continue
		}

		// Print the field
		fmt.Printf("  %s: %v\n", fieldName, fieldValue)
	}
}

// isEmptyValue checks if a value is empty
func isEmptyValue(v interface{}) bool {
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case string:
		return val == ""
	case []byte:
		return len(val) == 0
	default:
		return false
	}
}

// GetColor returns the appropriate color for a given status/level
func GetColor(status string) *color.Color {
	switch strings.ToUpper(status) {
	case "SUCCESS":
		return ColorGreen
	case "FAILED", "ERROR", "ERRORED":
		return ColorRed
	case "WARNING", "WARN":
		return ColorYellow
	case "DEBUG":
		return ColorBlue
	case "INFO":
		return ColorBrightCyan
	case "WORKFLOW": // Added for workflow events
		return ColorMagenta
	default:
		return ColorBrightCyan
	}
}

// HighlightLogLevels highlights INFO, WARN, ERROR in log messages
func HighlightLogLevels(msg string, levelColor *color.Color) string {
	// Replace level keywords with colored versions
	msg = strings.ReplaceAll(msg, "level=INFO", levelColor.Sprint("level=INFO"))
	msg = strings.ReplaceAll(msg, "level=WARN", levelColor.Sprint("level=WARN"))
	msg = strings.ReplaceAll(msg, "level=ERROR", levelColor.Sprint("level=ERROR"))
	msg = strings.ReplaceAll(msg, "level=DEBUG", levelColor.Sprint("level=DEBUG"))
	return msg
}

// GetLogLevel extracts log level from a message
func GetLogLevel(msg string) string {
	msgLower := strings.ToLower(msg)
	if strings.Contains(msgLower, "level=error") || strings.Contains(msgLower, "error") {
		return "ERROR"
	} else if strings.Contains(msgLower, "level=warn") || strings.Contains(msgLower, "warning") {
		return "WARN"
	} else if strings.Contains(msgLower, "level=debug") {
		return "DEBUG"
	}
	return "INFO"
}

// FormatStepRef formats step reference, handling the -1 case
func FormatStepRef(stepRef string) string {
	if stepRef == "-1" {
		return "0" // TODO: for some reason, stepRef is -1 for the first step?
	}
	return stepRef
}

// CleanLogMessage removes structured log patterns from messages
func CleanLogMessage(msg string) string {
	// Remove structured log patterns from the message
	// Common patterns: time=..., timestamp=..., ts=..., level=...
	msg = strings.TrimSpace(msg)

	// Remove time=... patterns
	timePattern := regexp.MustCompile(`time=\S+\s*`)
	msg = timePattern.ReplaceAllString(msg, "")

	// Remove timestamp=... patterns
	timestampPattern := regexp.MustCompile(`timestamp=\S+\s*`)
	msg = timestampPattern.ReplaceAllString(msg, "")

	// Remove ts=... patterns
	tsPattern := regexp.MustCompile(`ts=\S+\s*`)
	msg = tsPattern.ReplaceAllString(msg, "")

	// Remove level=... patterns
	levelPattern := regexp.MustCompile(`level=\S+\s*`)
	msg = levelPattern.ReplaceAllString(msg, "")

	return strings.TrimSpace(msg)
}

// FormatTimestamp converts RFC3339Nano timestamp to simple format
func FormatTimestamp(timestamp string) string {
	if t, err := time.Parse(time.RFC3339Nano, timestamp); err == nil {
		return t.Format("2006-01-02T15:04:05Z")
	}
	return timestamp
}

// FormatCapability formats capability ID for display
func FormatCapability(capabilityID string) string {
	if capabilityID == "" {
		return "unknown"
	}
	return capabilityID
}

// MapCapabilityStatus maps capability status to display format
func MapCapabilityStatus(status string) string {
	switch strings.ToLower(status) {
	case "success":
		return "SUCCESS"
	case "failed":
		return "FAILED"
	case "errored", "error":
		return "ERRORED"
	case "started":
		return "STARTED"
	default:
		return strings.ToUpper(status)
	}
}

// shouldEnableColors determines if colors should be enabled based on environment
func shouldEnableColors() bool {
	// Check if explicitly disabled
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check if explicitly enabled
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}

	// Check if we're in a CI environment (usually no colors)
	ciEnvs := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "JENKINS", "TRAVIS", "CIRCLECI"}
	for _, env := range ciEnvs {
		if os.Getenv(env) != "" {
			return false
		}
	}

	// Default to true - always enable colors for better user experience
	// Users can disable with --no-color or NO_COLOR=1
	return true
}

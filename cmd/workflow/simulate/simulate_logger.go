package simulate

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/smartcontractkit/cre-cli/internal/ui"
)

// LogLevel represents the level of a simulation log
type LogLevel string

const (
	LogLevelDebug   LogLevel = "DEBUG"
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarning LogLevel = "WARNING"
	LogLevelError   LogLevel = "ERROR"
)

// Style instances for consistent styling (using Chainlink Blocks palette)
var (
	StyleBlue       = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorBlue500))
	StyleBrightCyan = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ui.ColorTeal400))
	StyleYellow     = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorYellow400))
	StyleRed        = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorRed400))
	StyleGreen      = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorGreen400))
	StyleMagenta    = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorPurple400))
)

// SimulationLogger provides an easy interface for formatted simulation logs
type SimulationLogger struct {
	verbosity bool
}

// NewSimulationLogger creates a new simulation logger with verbosity control
func NewSimulationLogger(verbosity bool) *SimulationLogger {
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

	// Get style for the log level
	var levelStyle lipgloss.Style
	switch level {
	case LogLevelDebug:
		levelStyle = StyleBlue
	case LogLevelInfo:
		levelStyle = StyleBrightCyan
	case LogLevelWarning:
		levelStyle = StyleYellow
	case LogLevelError:
		levelStyle = StyleRed
	default:
		levelStyle = StyleBrightCyan
	}

	// Format with timestamp and level-specific style
	fmt.Printf("%s %s %s\n",
		StyleBlue.Render(timestamp),
		levelStyle.Render("[SIMULATION]"),
		formattedMessage)
}

// PrintTimestampedLog prints a log with timestamp and styled prefix
func (s *SimulationLogger) PrintTimestampedLog(timestamp, prefix, message string, prefixStyle lipgloss.Style) {
	fmt.Printf("%s %s %s\n",
		StyleBlue.Render(timestamp),
		prefixStyle.Render("["+prefix+"]"),
		message)
}

// PrintTimestampedLogWithStatus prints a log with timestamp, prefix, and styled status
func (s *SimulationLogger) PrintTimestampedLogWithStatus(timestamp, prefix, message, status string) {
	statusStyle := GetStyle(status)
	fmt.Printf("%s %s %s%s\n",
		StyleBlue.Render(timestamp),
		StyleMagenta.Render("["+prefix+"]"),
		message,
		statusStyle.Render(status))
}

// PrintStepLog prints a capability step log with timestamp and styled status
func (s *SimulationLogger) PrintStepLog(timestamp, component, stepRef, capability, status string) {
	statusStyle := GetStyle(status)
	fmt.Printf("%s %s       step[%s]   Capability: %s - %s\n",
		StyleBlue.Render(timestamp),
		StyleBrightCyan.Render("["+component+"]"),
		stepRef,
		capability,
		statusStyle.Render(status))
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

// GetStyle returns the appropriate style for a given status/level
func GetStyle(status string) lipgloss.Style {
	switch strings.ToUpper(status) {
	case "SUCCESS":
		return StyleGreen
	case "FAILED", "ERROR", "ERRORED":
		return StyleRed
	case "WARNING", "WARN":
		return StyleYellow
	case "DEBUG":
		return StyleBlue
	case "INFO":
		return StyleBrightCyan
	case "WORKFLOW": // Added for workflow events
		return StyleMagenta
	default:
		return StyleBrightCyan
	}
}

// HighlightLogLevels highlights INFO, WARN, ERROR in log messages
func HighlightLogLevels(msg string, levelStyle lipgloss.Style) string {
	// Replace level keywords with styled versions
	msg = strings.ReplaceAll(msg, "level=INFO", levelStyle.Render("level=INFO"))
	msg = strings.ReplaceAll(msg, "level=WARN", levelStyle.Render("level=WARN"))
	msg = strings.ReplaceAll(msg, "level=ERROR", levelStyle.Render("level=ERROR"))
	msg = strings.ReplaceAll(msg, "level=DEBUG", levelStyle.Render("level=DEBUG"))
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


package simulate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	pb "github.com/smartcontractkit/chainlink-protos/workflows/go/events"
)

// Color constants for consistent styling
const (
	COLOR_RESET       = "\033[0m"
	COLOR_RED         = "\033[91m" // Bright Red
	COLOR_GREEN       = "\033[32m" // Green
	COLOR_YELLOW      = "\033[33m" // Yellow
	COLOR_BLUE        = "\033[34m" // Blue
	COLOR_MAGENTA     = "\033[35m" // Magenta
	COLOR_CYAN        = "\033[36m" // Cyan
	COLOR_BRIGHT_CYAN = "\033[96m" // Bright Cyan
)

// entity types for clarity and organization
const (
	entityUserLogs           = "workflows.v1.UserLogs"
	entityWorkflowStarted    = "workflows.v1.WorkflowExecutionStarted"
	entityWorkflowFinished   = "workflows.v1.WorkflowExecutionFinished"
	entityCapabilityStarted  = "workflows.v1.CapabilityExecutionStarted"
	entityCapabilityFinished = "workflows.v1.CapabilityExecutionFinished"
)

// telemetryWriter is a custom writer that intercepts and parses beholder telemetry logs
type telemetryWriter struct {
	lggr                 logger.Logger
	verbose              bool
	failureEventReceived bool
}

// TelemetryLog represents the JSON structure of telemetry logs from beholder
type TelemetryLog struct {
	Severity int `json:"Severity"`
	Body     struct {
		Type  string `json:"Type"`
		Value string `json:"Value"` // Base64 encoded protobuf
	} `json:"Body"`
	Attributes []struct {
		Key   string `json:"Key"`
		Value struct {
			Type  string `json:"Type"`
			Value string `json:"Value"`
		} `json:"Value"`
	} `json:"Attributes"`
}

// MetricsLog represents OpenTelemetry metrics logs
type MetricsLog struct {
	Resource     []ResourceAttribute `json:"Resource"`
	ScopeMetrics []ScopeMetrics      `json:"ScopeMetrics"`
}

type ResourceAttribute struct {
	Key   string `json:"Key"`
	Value struct {
		Type  string `json:"Type"`
		Value string `json:"Value"`
	} `json:"Value"`
}

type ScopeMetrics struct {
	Scope   MetricsScope `json:"Scope"`
	Metrics []Metric     `json:"Metrics"`
}

type MetricsScope struct {
	Name    string `json:"Name"`
	Version string `json:"Version"`
}

type Metric struct {
	Name        string     `json:"Name"`
	Description string     `json:"Description"`
	Unit        string     `json:"Unit"`
	Data        MetricData `json:"Data"`
}

type MetricData struct {
	DataPoints []DataPoint `json:"DataPoints"`
}

type DataPoint struct {
	Attributes []MetricAttribute `json:"Attributes"`
	StartTime  string            `json:"StartTime"`
	Time       string            `json:"Time"`
	Value      interface{}       `json:"Value"`
}

type MetricAttribute struct {
	Key   string `json:"Key"`
	Value struct {
		Type  string `json:"Type"`
		Value string `json:"Value"`
	} `json:"Value"`
}

// Write implements io.Writer interface
func (w *telemetryWriter) Write(p []byte) (n int, err error) {
	// Try to parse as beholder telemetry log first
	var telLog TelemetryLog
	if err := json.Unmarshal(p, &telLog); err == nil && len(telLog.Attributes) > 0 {
		// Check what type of beholder event this is
		entityType := ""
		for _, attr := range telLog.Attributes {
			if attr.Key == "beholder_entity" {
				entityType = attr.Value.Value
				break
			}
		}

		// When not verbose, only print UserLogs and skip the rest
		if !w.verbose && entityType != entityUserLogs {
			return len(p), nil
		}

		// Handle different types of telemetry logs
		switch entityType {
		case entityUserLogs:
			w.handleUserLogs(telLog)
			return len(p), nil
		case "BaseMessage":
			return 0, nil
		case entityWorkflowStarted:
			w.handleWorkflowEvent(telLog, "started")
			return len(p), nil
		case entityWorkflowFinished:
			w.handleWorkflowEvent(telLog, "finished")
			return len(p), nil
		case entityCapabilityStarted:
			w.handleCapabilityEvent(telLog, "started")
			return len(p), nil
		case entityCapabilityFinished:
			w.handleCapabilityEvent(telLog, "finished")
			return len(p), nil
		}
	}

	// Try to parse as OpenTelemetry metrics log
	var metricsLog MetricsLog
	if err := json.Unmarshal(p, &metricsLog); err == nil && len(metricsLog.Resource) > 0 {
		return 0, nil
	}

	// Unknown format: only pass through raw when verbose
	if w.verbose {
		w.lggr.Info(string(p))
	}
	return len(p), nil
}

// handleUserLogs processes UserLogs telemetry events
func (w *telemetryWriter) handleUserLogs(telLog TelemetryLog) {
	if telLog.Body.Type == "Bytes" && telLog.Body.Value != "" {
		decoded, err := base64.StdEncoding.DecodeString(telLog.Body.Value)
		if err != nil {
			w.lggr.Errorf("Failed to decode userLogs body: %v", err)
			return
		}

		var userLogs pb.UserLogs
		if err := proto.Unmarshal(decoded, &userLogs); err != nil {
			w.lggr.Errorf("Failed to unmarshal UserLogs protobuf: %v", err)
			return
		}

		// Format and display the user logs
		w.formatUserLogs(&userLogs)
	}
}

// handleWorkflowEvent processes workflow execution events (started/finished)
func (w *telemetryWriter) handleWorkflowEvent(telLog TelemetryLog, eventType string) {
	if telLog.Body.Type == "Bytes" && telLog.Body.Value != "" {
		decoded, err := base64.StdEncoding.DecodeString(telLog.Body.Value)
		if err != nil {
			w.lggr.Errorf("Failed to decode workflow event body: %v", err)
			return
		}

		if eventType == "started" {
			// Handle started event
			var workflowEvent pb.WorkflowExecutionStarted
			if err := proto.Unmarshal(decoded, &workflowEvent); err != nil {
				w.lggr.Errorf("Failed to unmarshal workflow started event: %v", err)
				return
			}
			timestamp := formatTimestamp(workflowEvent.Timestamp)
			fmt.Printf("%s%s%s %s[WORKFLOW]%s WorkflowExecutionStarted\n",
				COLOR_BLUE, timestamp, COLOR_RESET, COLOR_MAGENTA, COLOR_RESET)
			// Display trigger information
			if workflowEvent.TriggerID != "" {
				fmt.Printf("  TriggerID: %s\n", workflowEvent.TriggerID)
			}
			// Display workflow metadata if available
			if workflowEvent.M != nil {
				if workflowEvent.M.WorkflowName != "" {
					fmt.Printf("  WorkflowName: %s\n", workflowEvent.M.WorkflowName)
				}
				if workflowEvent.M.WorkflowID != "" {
					fmt.Printf("  WorkflowID: %s\n", workflowEvent.M.WorkflowID)
				}
				if workflowEvent.M.WorkflowExecutionID != "" {
					fmt.Printf("  ExecutionID: %s\n", workflowEvent.M.WorkflowExecutionID)
				}
				if workflowEvent.M.WorkflowOwner != "" {
					fmt.Printf("  WorkflowOwner: %s\n", workflowEvent.M.WorkflowOwner)
				}
				if workflowEvent.M.Version != "" {
					fmt.Printf("  Version: %s\n", workflowEvent.M.Version)
				}
			}
		} else {
			// Handle finished event
			var finishedEvent pb.WorkflowExecutionFinished
			if err := proto.Unmarshal(decoded, &finishedEvent); err != nil {
				w.lggr.Errorf("Failed to unmarshal workflow finished event: %v", err)
				return
			}
			timestamp := formatTimestamp(finishedEvent.Timestamp)
			status := w.mapWorkflowStatus(finishedEvent.Status)
			color := getColor(status)
			fmt.Printf("%s%s%s %s[WORKFLOW]%s WorkflowExecutionFinished - Status: %s%s%s\n",
				COLOR_BLUE, timestamp, COLOR_RESET, COLOR_MAGENTA, COLOR_RESET, color, status, COLOR_RESET)
			// Display additional workflow metadata if available
			if finishedEvent.M != nil {
				if finishedEvent.M.WorkflowName != "" {
					fmt.Printf("  WorkflowName: %s\n", finishedEvent.M.WorkflowName)
				}
				if finishedEvent.M.WorkflowID != "" {
					fmt.Printf("  WorkflowID: %s\n", finishedEvent.M.WorkflowID)
				}
				if finishedEvent.M.WorkflowExecutionID != "" {
					fmt.Printf("  ExecutionID: %s\n", finishedEvent.M.WorkflowExecutionID)
				}
				if finishedEvent.M.WorkflowOwner != "" {
					fmt.Printf("  WorkflowOwner: %s\n", finishedEvent.M.WorkflowOwner)
				}
				if finishedEvent.M.Version != "" {
					fmt.Printf("  Version: %s\n", finishedEvent.M.Version)
				}
			}
		}
	}
}

// handleCapabilityEvent processes capability execution events (started/finished)
func (w *telemetryWriter) handleCapabilityEvent(telLog TelemetryLog, eventType string) {
	if telLog.Body.Type == "Bytes" && telLog.Body.Value != "" {
		decoded, err := base64.StdEncoding.DecodeString(telLog.Body.Value)
		if err != nil {
			w.lggr.Errorf("Failed to decode capability event body: %v", err)
			return
		}

		if eventType == "started" {
			// Handle started event
			var capEvent pb.CapabilityExecutionStarted
			if err := proto.Unmarshal(decoded, &capEvent); err != nil {
				w.lggr.Errorf("Failed to unmarshal capability started event: %v", err)
				return
			}
			timestamp := formatTimestamp(capEvent.Timestamp)
			capability := formatCapability(capEvent.CapabilityID)
			stepRef := formatStepRef(capEvent.StepRef)
			fmt.Printf("%s%s%s %s[STARTED]%s         step[%s]   Capability: %s\n",
				COLOR_BLUE, timestamp, COLOR_RESET, COLOR_YELLOW, COLOR_RESET, stepRef, capability)
		} else {
			// Handle finished event
			var finishedEvent pb.CapabilityExecutionFinished
			if err := proto.Unmarshal(decoded, &finishedEvent); err != nil {
				w.lggr.Errorf("Failed to unmarshal capability finished event: %v", err)
				return
			}
			timestamp := formatTimestamp(finishedEvent.Timestamp)
			capability := formatCapability(finishedEvent.CapabilityID)
			status := mapCapabilityStatus(finishedEvent.Status)
			if status == "FAILED" || status == "ERRORED" {
				w.failureEventReceived = true
			}
			color := getColor(status)
			stepRef := formatStepRef(finishedEvent.StepRef)
			fmt.Printf("%s%s%s %s[%s]%s         step[%s]   Capability: %s\n",
				COLOR_BLUE, timestamp, COLOR_RESET, color, status, COLOR_RESET, stepRef, capability)
		}
	}
}

// formatUserLogs formats and displays user logs in a readable way
func (w *telemetryWriter) formatUserLogs(logs *pb.UserLogs) {
	// Display each log line
	for _, logLine := range logs.LogLines {
		timestamp := ""
		if logLine.NodeTimestamp != "" {
			// Try to parse and format timestamp
			if t, err := time.Parse(time.RFC3339Nano, logLine.NodeTimestamp); err == nil {
				timestamp = t.Format("2006-01-02T15:04:05Z")
			} else {
				timestamp = logLine.NodeTimestamp
			}
		}

		// Format the log message
		level := getLogLevel(logLine.Message)
		msg := cleanLogMessage(logLine.Message)
		levelColor := getColor(level)

		// Highlight level keywords in the message
		highlightedMsg := highlightLogLevels(msg, levelColor)

		if timestamp != "" {
			fmt.Printf("%s%s%s %s[USER LOG]%s %s\n", COLOR_BLUE, timestamp, COLOR_RESET, levelColor, COLOR_RESET, highlightedMsg)
		} else {
			fmt.Printf("%s[USER LOG]%s %s\n", levelColor, COLOR_RESET, highlightedMsg)
		}
	}
}

// Helper functions for formatting

func getLogLevel(msg string) string {
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

func getColor(level string) string {
	switch level {
	case "ERROR":
		return COLOR_RED
	case "WARN":
		return COLOR_YELLOW
	case "INFO":
		return COLOR_BRIGHT_CYAN
	case "DEBUG":
		return COLOR_BLUE
	case "SUCCESS":
		return COLOR_GREEN
	case "FAILED", "ERRORED":
		return COLOR_RED
	case "STARTED":
		return COLOR_YELLOW
	case "COMPLETED":
		return COLOR_CYAN
	default:
		return COLOR_CYAN
	}
}

func formatStepRef(stepRef string) string {
	if stepRef == "-1" {
		return "0" // TODO: for some reason, stepRef is -1 for the first step?
	}
	return stepRef
}

func cleanLogMessage(msg string) string {
	// Just return the message as-is, since it's already the actual log content
	// The protobuf UserLogs.LogLine.Message field contains the clean message
	return strings.TrimSpace(msg)
}

// formatTimestamp converts RFC3339Nano timestamp to simple format
func formatTimestamp(timestamp string) string {
	if t, err := time.Parse(time.RFC3339Nano, timestamp); err == nil {
		return t.Format("2006-01-02T15:04:05Z")
	}
	return timestamp
}

// formatCapability extracts capability name from full ID
func formatCapability(capabilityID string) string {
	// Extract just the capability name (e.g., "http-actions@0.1.0"
	return capabilityID
}

// mapCapabilityStatus maps internal status to display format (for capabilities)
func mapCapabilityStatus(status string) string {
	switch strings.ToLower(status) {
	case "completed":
		return "SUCCESS"
	case "success":
		return "SUCCESS"
	case "failed", "error":
		return "FAILED"
	case "errored":
		return "ERRORED"
	case "started":
		return "STARTED"
	default:
		return strings.ToUpper(status)
	}
}

// mapWorkflowStatus maps workflow status to display format (different from capability status)
// TODO: workflow status returns "completed" regardless of success or failure, which is misleading
func (w *telemetryWriter) mapWorkflowStatus(status string) string {
	switch strings.ToLower(status) {
	case "completed":
		if w.failureEventReceived {
			return "FAILED"
		}
		return "SUCCESS"
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

// highlightLogLevels highlights INFO, WARN, ERROR in log messages
func highlightLogLevels(msg, levelColor string) string {
	// Replace level keywords with colored versions
	msg = strings.ReplaceAll(msg, "level=INFO", levelColor+"level=INFO"+COLOR_RESET)
	msg = strings.ReplaceAll(msg, "level=WARN", levelColor+"level=WARN"+COLOR_RESET)
	msg = strings.ReplaceAll(msg, "level=ERROR", levelColor+"level=ERROR"+COLOR_RESET)
	msg = strings.ReplaceAll(msg, "level=DEBUG", levelColor+"level=DEBUG"+COLOR_RESET)
	return msg
}

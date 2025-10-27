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
	simLogger            *SimulationLogger
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
			return len(p), nil
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
			timestamp := FormatTimestamp(workflowEvent.Timestamp)
			w.simLogger.PrintTimestampedLog(timestamp, "WORKFLOW", "WorkflowExecutionStarted", ColorMagenta)

			// Display trigger information
			if workflowEvent.TriggerID != "" {
				fmt.Printf("  TriggerID: %s\n", workflowEvent.TriggerID)
			}
			// Display workflow metadata if available
			w.simLogger.PrintWorkflowMetadata(workflowEvent.M)
		} else {
			// Handle finished event
			var finishedEvent pb.WorkflowExecutionFinished
			if err := proto.Unmarshal(decoded, &finishedEvent); err != nil {
				w.lggr.Errorf("Failed to unmarshal workflow finished event: %v", err)
				return
			}
			timestamp := FormatTimestamp(finishedEvent.Timestamp)
			status := w.mapWorkflowStatus(finishedEvent.Status)
			w.simLogger.PrintTimestampedLogWithStatus(timestamp, "WORKFLOW", "WorkflowExecutionFinished - Status: ", status)

			// Display additional workflow metadata if available
			w.simLogger.PrintWorkflowMetadata(finishedEvent.M)
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
			timestamp := FormatTimestamp(capEvent.Timestamp)
			capability := FormatCapability(capEvent.CapabilityID)
			stepRef := FormatStepRef(capEvent.StepRef)
			w.simLogger.PrintStepLog(timestamp, "SIMULATOR", stepRef, capability, "STARTED")
		} else {
			// Handle finished event
			var finishedEvent pb.CapabilityExecutionFinished
			if err := proto.Unmarshal(decoded, &finishedEvent); err != nil {
				w.lggr.Errorf("Failed to unmarshal capability finished event: %v", err)
				return
			}
			timestamp := FormatTimestamp(finishedEvent.Timestamp)
			capability := FormatCapability(finishedEvent.CapabilityID)
			status := MapCapabilityStatus(finishedEvent.Status)
			if status == "FAILED" || status == "ERRORED" {
				w.failureEventReceived = true
			}
			stepRef := FormatStepRef(finishedEvent.StepRef)
			w.simLogger.PrintStepLog(timestamp, "SIMULATOR", stepRef, capability, status)
		}
	}
}

// formatUserLogs formats and displays user logs in a readable way
func (w *telemetryWriter) formatUserLogs(logs *pb.UserLogs) {
	// Display each log line
	for _, logLine := range logs.LogLines {
		// Format the log message
		level := GetLogLevel(logLine.Message)
		msg := CleanLogMessage(logLine.Message)
		levelColor := GetColor(level)

		// Highlight level keywords in the message
		highlightedMsg := HighlightLogLevels(msg, levelColor)

		// Always use current timestamp for consistency with other logs
		w.simLogger.PrintTimestampedLog(time.Now().Format("2006-01-02T15:04:05Z"), "USER LOG", highlightedMsg, ColorBrightCyan)
	}
}

// Helper functions for formatting

// mapWorkflowStatus maps workflow status to display format (different from capability status)
func (w *telemetryWriter) mapWorkflowStatus(status string) string {
	switch strings.ToLower(status) {
	case "completed":
		// workflow is in completed status, but if any failure events were received, mark as FAILED
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

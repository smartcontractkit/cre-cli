package logger

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/smartcontractkit/chainlink-testing-framework/seth"
)

func TestLogger(t *testing.T) {
	t.Run("New creates working logger", func(t *testing.T) {
		// Create a buffer to capture output
		var buf bytes.Buffer

		// Create logger writing to buffer
		log := New(
			WithOutput(&buf),
			WithLevel("debug"),
			WithConsoleWriter(false), // Disable pretty printing for test
		)

		// Write a test message
		log.Info().Msg("test message")

		// Verify output contains the message
		output := buf.String()
		assert.Contains(t, output, "test message")
		assert.Contains(t, output, "info")
	})

	t.Run("Logger respects log levels", func(t *testing.T) {
		var buf bytes.Buffer
		log := New(
			WithOutput(&buf),
			WithLevel("info"),
			WithConsoleWriter(false),
		)

		// Debug shouldn't be logged at info level
		log.Debug().Msg("debug message")
		assert.Empty(t, buf.String(), "Debug message should not be logged")

		// Info should be logged
		log.Info().Msg("info message")
		assert.Contains(t, buf.String(), "info message")
	})

	t.Run("Development mode enables pretty logging", func(t *testing.T) {
		var buf bytes.Buffer
		log := New(
			WithOutput(&buf),
			WithLevel("debug"),
			WithConsoleWriter(true),
		)

		log.Info().Msg("pretty message")
		output := buf.String()

		// Console writer uses level abbreviations like "INF" instead of JSON format
		assert.Contains(t, output, "INF")
		// Console format should not be JSON
		assert.NotContains(t, output, `{"level":"info"}`)
	})

	t.Run("Logger with fields", func(t *testing.T) {
		var buf bytes.Buffer
		log := *New(
			WithOutput(&buf),
			WithLevel("debug"),
			WithConsoleWriter(false),
		)

		// Add fields
		log = log.With().
			Str("service", "test").
			Int("attempt", 1).
			Logger()

		log.Info().Msg("test with fields")
		output := buf.String()

		assert.Contains(t, output, "service")
		assert.Contains(t, output, "test")
		assert.Contains(t, output, "attempt")
		assert.Contains(t, output, "1")
	})

	t.Run("Logs workflow ID as a hex string when parsing EventData map", func(t *testing.T) {
		var buf bytes.Buffer
		log := New(
			WithOutput(&buf),
			WithLevel("debug"),
			WithConsoleWriter(false),
		)

		eventData := map[string]interface{}{
			"WorkflowID":    [32]byte{1, 2, 3},
			"WorkflowName":  "Sample Workflow",
			"WorkflowOwner": "0x1234567890abcdef1234567890abcdef12345678",
			"Raw":           map[string]interface{}{"key": "value"},
		}
		testLogEvent := seth.DecodedCommonLog{
			Signature: "WorkflowTestEventV1(address,uint32,string)",
			Address:   common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
			EventData: eventData,
			Topics:    []string{},
		}

		log.Info().
			Object("Event Data", EventDataWrapper{EventData: testLogEvent.EventData}).
			Msg("Event emitted")
		output := buf.String()

		assert.Contains(t, output, "Event emitted")
		assert.Contains(t, output, `"WorkflowID":"0102030000000000000000000000000000000000000000000000000000000000"`)
		assert.Contains(t, output, `"WorkflowName":"Sample Workflow"`)
		assert.Contains(t, output, `"WorkflowOwner":"0x1234567890abcdef1234567890abcdef12345678"`)
		assert.Contains(t, output, `"Raw":{"key":"value"}`)
	})

	t.Run("Logs WorkflowID as a hex string in DecodedTransactionLog", func(t *testing.T) {
		var buf bytes.Buffer
		log := New(
			WithOutput(&buf),
			WithLevel("debug"),
			WithConsoleWriter(false),
		)

		eventData := map[string]interface{}{
			"WorkflowID":   [32]byte{1, 2, 3},
			"WorkflowName": "Sample Workflow",
		}
		testLog := seth.DecodedTransactionLog{
			DecodedCommonLog: seth.DecodedCommonLog{
				Signature: "WorkflowTestEventV1(address,uint32,string)",
				Address:   common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678"),
				EventData: eventData,
				Topics:    []string{"topic1", "topic2"},
			},
			BlockNumber: 1234567,
			Index:       1,
			TXHash:      "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef",
			TXIndex:     0,
			Removed:     false,
			FileTag:     "",
		}

		log.Info().
			Object("Event", DecodedTransactionLogWrapper{DecodedTransactionLog: testLog}).
			Msg("Found event")
		output := buf.String()

		assert.Contains(t, output, "Found event")
		assert.Contains(t, output, `"BlockNumber":1234567`)
		assert.Contains(t, output, `"Index":1`)
		assert.Contains(t, output, `"TxHash":"0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef"`)
		assert.Contains(t, output, `"TxIndex":0`)
		assert.Contains(t, output, `"Removed":false`)
		assert.Contains(t, output, `"FileTag":""`)

		assert.Contains(t, output, `"Signature":"WorkflowTestEventV1(address,uint32,string)"`)
		assert.Contains(t, output, `"Address":"0x1234567890AbcdEF1234567890aBcdef12345678"`)
		assert.Contains(t, output, `"Topics":["topic1","topic2"]`)

		assert.Contains(t, output, `"WorkflowID":"0102030000000000000000000000000000000000000000000000000000000000"`)
		assert.Contains(t, output, `"WorkflowName":"Sample Workflow"`)
	})

}

// TestLogOutput is a helper to capture and verify log output
func TestLogOutput(t *testing.T) {
	tests := []struct {
		name     string
		logFunc  func(*zerolog.Logger)
		level    string
		contains []string
	}{
		{
			name: "info level",
			logFunc: func(log *zerolog.Logger) {
				log.Info().Str("key", "value").Msg("info message")
			},
			level:    "info",
			contains: []string{"info message", "key", "value"},
		},
		{
			name: "error level",
			logFunc: func(log *zerolog.Logger) {
				log.Error().Err(assert.AnError).Msg("error message")
			},
			level:    "error",
			contains: []string{"error message", "error", assert.AnError.Error()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := New(
				WithOutput(&buf),
				WithLevel(tt.level),
				WithConsoleWriter(false),
			)

			tt.logFunc(log)
			output := buf.String()

			for _, str := range tt.contains {
				assert.Contains(t, output, str)
			}
		})
	}
}

// Benchmark logging performance
func BenchmarkLogger(b *testing.B) {
	var buf bytes.Buffer
	log := New(
		WithOutput(&buf),
		WithLevel("info"),
		WithConsoleWriter(false),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info().Int("count", i).Msg("benchmark message")
	}
}

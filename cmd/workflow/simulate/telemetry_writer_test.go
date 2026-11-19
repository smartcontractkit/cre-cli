package simulate

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	pb "github.com/smartcontractkit/chainlink-protos/workflows/go/events"
	pbv2 "github.com/smartcontractkit/chainlink-protos/workflows/go/v2"
)

// telemetryRecord builds the JSON a beholder stdout exporter writes for a proto event.
func telemetryRecord(t *testing.T, entity string, msg proto.Message) []byte {
	t.Helper()
	body, err := proto.Marshal(msg)
	require.NoError(t, err)

	rec := map[string]any{
		"Severity": 0,
		"Body": map[string]any{
			"Type":  "BYTESLICE",
			"Value": base64.StdEncoding.EncodeToString(body),
		},
		"Attributes": []any{
			map[string]any{"Key": "beholder_entity", "Value": map[string]any{"Type": "STRING", "Value": entity}},
		},
	}
	out, err := json.Marshal(rec)
	require.NoError(t, err)
	return out
}

// captureStdout runs fn and returns everything it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func newTestTelemetryWriter(verbose bool) *telemetryWriter {
	return &telemetryWriter{
		lggr:      logger.Nop(),
		verbose:   verbose,
		simLogger: NewSimulationLogger(verbose),
	}
}

// Engines that emit only the v2 schema deliver user logs as workflows.v2.WorkflowUserLog.
// Without handling it, every workflow log line vanishes from simulation output.
func TestTelemetryWriter_UserLogsV2(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		w := newTestTelemetryWriter(verbose)
		rec := telemetryRecord(t, entityUserLogsV2, &pbv2.WorkflowUserLog{Msg: "hello from workflow"})

		var n int
		out := captureStdout(t, func() {
			var err error
			n, err = w.Write(rec)
			require.NoError(t, err)
		})

		assert.Equal(t, len(rec), n, "verbose=%v", verbose)
		assert.Contains(t, out, "[USER LOG]", "verbose=%v", verbose)
		assert.Contains(t, out, "hello from workflow", "verbose=%v", verbose)
	}
}

func TestTelemetryWriter_UserLogsV1StillSupported(t *testing.T) {
	w := newTestTelemetryWriter(false)
	rec := telemetryRecord(t, entityUserLogs, &pb.UserLogs{LogLines: []*pb.LogLine{{Message: "legacy line"}}})

	out := captureStdout(t, func() {
		_, err := w.Write(rec)
		require.NoError(t, err)
	})

	assert.Contains(t, out, "[USER LOG]")
	assert.Contains(t, out, "legacy line")
}

func TestTelemetryWriter_UserLogsV2BadBodyDoesNotPanic(t *testing.T) {
	w := newTestTelemetryWriter(false)
	rec := []byte(`{"Severity":0,"Body":{"Type":"BYTESLICE","Value":"!!not-base64!!"},"Attributes":[{"Key":"beholder_entity","Value":{"Type":"STRING","Value":"workflows.v2.WorkflowUserLog"}}]}`)

	out := captureStdout(t, func() {
		n, err := w.Write(rec)
		require.NoError(t, err)
		assert.Equal(t, len(rec), n)
	})
	assert.NotContains(t, out, "[USER LOG]")
}

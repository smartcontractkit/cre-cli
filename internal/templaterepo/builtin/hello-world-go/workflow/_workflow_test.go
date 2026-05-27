package main

import (
	"strings"
	"testing"
	"time"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
	"github.com/smartcontractkit/cre-sdk-go/cre/testutils"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var anyExecutionTime = time.Date(2025, 7, 14, 17, 41, 57, 0, time.UTC)

func TestInitWorkflow(t *testing.T) {
	config := &Config{}
	runtime := testutils.NewRuntime(t, testutils.Secrets{})

	workflow, err := InitWorkflow(config, runtime.Logger(), nil)
	require.NoError(t, err)

	require.Len(t, workflow, 1)
	require.Equal(t, cron.Trigger(&cron.Config{}).CapabilityID(), workflow[0].CapabilityID())
}

func TestOnCronTrigger(t *testing.T) {
	config := &Config{}
	runtime := testutils.NewRuntime(t, testutils.Secrets{})

	payload := &cron.Payload{
		ScheduledExecutionTime: timestamppb.New(anyExecutionTime),
	}

	result, err := onCronTrigger(config, runtime, payload)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Result, "Fired at")
	require.Contains(t, result.Result, "2025-07-14")

	logs := runtime.GetLogs()
	assertLogContains(t, logs, "Cron trigger fired")
}

func assertLogContains(t *testing.T, logs [][]byte, substr string) {
	t.Helper()
	for _, line := range logs {
		if strings.Contains(string(line), substr) {
			return
		}
	}
	var logStrings []string
	for _, log := range logs {
		logStrings = append(logStrings, string(log))
	}
	t.Fatalf("Expected logs to contain substring %q, but it was not found in logs:\n%s",
		substr, strings.Join(logStrings, "\n"))
}

package main

import (
	"fmt"
	"log/slog"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
	"github.com/smartcontractkit/cre-sdk-go/cre"
)

type ExecutionResult struct {
	Result string
}

// Workflow configuration loaded from the config.json file
type Config struct{}

// Workflow implementation with a list of capability triggers
func InitWorkflow(config *Config, logger *slog.Logger, secretsProvider cre.SecretsProvider) (cre.Workflow[*Config], error) {
	// Create the trigger
	cronTrigger := cron.Trigger(&cron.Config{Schedule: "*/30 * * * * *"}) // Fires every 30 seconds

	// Register a handler with the trigger and a callback function
	return cre.Workflow[*Config]{
		cre.Handler(cronTrigger, onCronTrigger),
	}, nil
}

func onCronTrigger(config *Config, runtime cre.Runtime, trigger *cron.Payload) (*ExecutionResult, error) {
	logger := runtime.Logger()
	scheduledTime := trigger.ScheduledExecutionTime.AsTime()
	logger.Info("Cron trigger fired", "scheduledTime", scheduledTime)

	// Your logic here...

	return &ExecutionResult{Result: fmt.Sprintf("Fired at %s", scheduledTime)}, nil
}

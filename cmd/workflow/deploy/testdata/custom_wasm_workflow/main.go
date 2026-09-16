//go:build wasip1

package main

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/cre-sdk-go/cre/wasm"
)

type Config struct {
	WorkflowName  string `json:"workflowName"`
	WorkflowOwner string `json:"workflowOwner"`
	Schedule      string `json:"schedule"`
}

func InitWorkflow(config *Config, logger *slog.Logger, secretsProvider cre.SecretsProvider) (cre.Workflow[*Config], error) {
	if config.Schedule == "" {
		return nil, errors.New("schedule is missing in the workflow config")
	}

	cronTrigger := cron.Trigger(&cron.Config{Schedule: config.Schedule})

	return cre.Workflow[*Config]{
		cre.Handler(cronTrigger, onCronTrigger),
	}, nil
}

func onCronTrigger(config *Config, runtime cre.Runtime, trigger *cron.Payload) (string, error) {
	logger := runtime.Logger()
	scheduledTime := trigger.ScheduledExecutionTime.AsTime()
	logger.Info("Cron trigger fired", "workflowName", config.WorkflowName, "scheduledTime", scheduledTime)

	if config.WorkflowOwner == "" {
		return "", errors.New("it is cool, not good")
	}

	return fmt.Sprintf("Fired at %s", scheduledTime), nil
}

func main() {
	wasm.NewRunner(cre.ParseJSON[Config]).Run(InitWorkflow)
}

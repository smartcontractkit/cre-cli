package telemetry

import (
	"context"
	"io"
	"time"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

const (
	sendTimeout = 5 * time.Second
)

const reportUserEventMutation = `
mutation ReportUserEvent($event: UserEventInput!) {
  reportUserEvent(event: $event) {
    success
    message
  }
}
`

// SendEvent sends a user event to the GraphQL endpoint
// All errors are silently swallowed unless debug logging is enabled
func SendEvent(ctx context.Context, event UserEventInput, creds *credentials.Credentials, envSet *environments.EnvironmentSet, logger *zerolog.Logger) {
	// Create context with timeout
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	// Recover from any panics
	defer func() {
		if r := recover(); r != nil {
			DebugLog("sender panic: %v", r)
		}
	}()

	// Skip if no credentials (not authenticated)
	if creds == nil {
		DebugLog("skipping telemetry: no credentials")
		return
	}

	// Skip if no environment set
	if envSet == nil {
		DebugLog("skipping telemetry: no environment set")
		return
	}

	// Use provided logger if available and debug is enabled, otherwise use silent logger
	var clientLogger *zerolog.Logger
	// --- FIX: Use public IsTelemetryDebugEnabled ---
	if IsTelemetryDebugEnabled() && logger != nil {
		DebugLog("using provided logger for GraphQL client")
		clientLogger = logger
	} else {
		silentLogger := zerolog.New(io.Discard)
		clientLogger = &silentLogger
	}

	DebugLog("creating GraphQL client for endpoint: %s", envSet.GraphQLURL)
	client := graphqlclient.New(creds, envSet, clientLogger)

	// Create the GraphQL request
	DebugLog("creating GraphQL request with mutation")
	req := graphql.NewRequest(reportUserEventMutation)
	req.Var("event", event)

	// Execute the request
	DebugLog("executing GraphQL request")
	var resp ReportUserEventResponse
	err := client.Execute(sendCtx, req, &resp)

	if err != nil {
		DebugLog("telemetry request failed: %v", err)
	} else {
		DebugLog("telemetry request succeeded: success=%v, message=%s", resp.ReportUserEvent.Success, resp.ReportUserEvent.Message)
	}
}

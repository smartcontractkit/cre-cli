package workflowdataclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/machinebox/graphql"
)

// ExecutionStatus mirrors the WorkflowExecutionStatus enum from the platform schema.
type ExecutionStatus string

const (
	ExecutionStatusUnknown     ExecutionStatus = "UNKNOWN"
	ExecutionStatusUnspecified ExecutionStatus = "UNSPECIFIED"
	ExecutionStatusTriggered   ExecutionStatus = "TRIGGERED"
	ExecutionStatusInProgress  ExecutionStatus = "IN_PROGRESS"
	ExecutionStatusSuccess     ExecutionStatus = "SUCCESS"
	ExecutionStatusFailure     ExecutionStatus = "FAILURE"
)

// ValidExecutionStatuses is the full set of values accepted by the platform.
var ValidExecutionStatuses = []ExecutionStatus{
	ExecutionStatusTriggered,
	ExecutionStatusInProgress,
	ExecutionStatusSuccess,
	ExecutionStatusFailure,
}

// ExecutionError is a top-level error on a workflow execution.
type ExecutionError struct {
	Error string
	Count int
}

// Execution is a single workflow execution record.
type Execution struct {
	UUID         string
	ID           string // on-chain execution ID shown in the Explorer UI
	WorkflowUUID string
	WorkflowID   string // on-chain workflow hash (workflowId scalar)
	WorkflowName string
	Status       ExecutionStatus
	StartedAt    time.Time
	FinishedAt   *time.Time
	CreditUsed   *string // CreditAmount scalar serialised as a string
	Errors       []ExecutionError
}

// CapabilityExecutionError is an error attached to a capability event.
type CapabilityExecutionError struct {
	Error string
	Count int
}

// ExecutionEvent is one node/capability event within an execution.
type ExecutionEvent struct {
	CapabilityID string
	Status       string
	StartedAt    time.Time
	FinishedAt   *time.Time
	Errors       []CapabilityExecutionError
	Method       *string
}

// ExecutionLog is a single log line emitted during an execution.
type ExecutionLog struct {
	NodeID    string
	Message   string
	Timestamp time.Time
}

// ListExecutionsInput maps to WorkflowExecutionsInput on the platform.
type ListExecutionsInput struct {
	WorkflowUUID *string
	Statuses     []ExecutionStatus
	From         *time.Time
	To           *time.Time
	// Limit is the maximum number of results to return (capped at 100 by the API).
	Limit int
}

// ListEventsInput maps to WorkflowExecutionEventsInput on the platform.
type ListEventsInput struct {
	ExecutionUUID string
	CapabilityID  *string
	Status        *string
}

// ---- GraphQL query strings ----

const listExecutionsQuery = `
query ListExecutions($input: WorkflowExecutionsInput!) {
  workflowExecutions(input: $input) {
    data {
      uuid
      id
      workflowUUID
      workflowId
      workflowName
      status
      startedAt
      finishedAt
      creditUsed
      errors {
        error
        count
      }
    }
    count
  }
}
`

const getExecutionQuery = `
query GetExecution($input: WorkflowExecutionInput!) {
  workflowExecution(input: $input) {
    data {
      uuid
      id
      workflowUUID
      workflowId
      workflowName
      status
      startedAt
      finishedAt
      creditUsed
      errors {
        error
        count
      }
    }
  }
}
`

const listExecutionEventsQuery = `
query ListExecutionEvents($input: WorkflowExecutionEventsInput!) {
  workflowExecutionEvents(input: $input) {
    data {
      capabilityID
      status
      startedAt
      finishedAt
      method
      errors {
        error
        count
      }
    }
  }
}
`

const listExecutionLogsQuery = `
query ListExecutionLogs($input: WorkflowExecutionLogsInput!) {
  workflowExecutionLogs(input: $input) {
    data {
      nodeID
      message
      timestamp
    }
  }
}
`

// ---- GQL envelope types ----

type gqlExecutionError struct {
	Error string `json:"error"`
	Count int    `json:"count"`
}

type gqlExecution struct {
	UUID         string              `json:"uuid"`
	ID           string              `json:"id"`
	WorkflowUUID string              `json:"workflowUUID"`
	WorkflowID   string              `json:"workflowId"`
	WorkflowName string              `json:"workflowName"`
	Status       string              `json:"status"`
	StartedAt    time.Time           `json:"startedAt"`
	FinishedAt   *time.Time          `json:"finishedAt"`
	CreditUsed   *string             `json:"creditUsed"`
	Errors       []gqlExecutionError `json:"errors"`
}

type listExecutionsEnvelope struct {
	WorkflowExecutions struct {
		Data  []gqlExecution `json:"data"`
		Count int            `json:"count"`
	} `json:"workflowExecutions"`
}

type getExecutionEnvelope struct {
	WorkflowExecution struct {
		Data *gqlExecution `json:"data"`
	} `json:"workflowExecution"`
}

type gqlCapabilityError struct {
	Error string `json:"error"`
	Count int    `json:"count"`
}

type gqlExecutionEvent struct {
	CapabilityID string               `json:"capabilityID"`
	Status       string               `json:"status"`
	StartedAt    time.Time            `json:"startedAt"`
	FinishedAt   *time.Time           `json:"finishedAt"`
	Method       *string              `json:"method"`
	Errors       []gqlCapabilityError `json:"errors"`
}

type listEventsEnvelope struct {
	WorkflowExecutionEvents struct {
		Data []gqlExecutionEvent `json:"data"`
	} `json:"workflowExecutionEvents"`
}

type gqlExecutionLog struct {
	NodeID    string    `json:"nodeID"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type listLogsEnvelope struct {
	WorkflowExecutionLogs struct {
		Data []gqlExecutionLog `json:"data"`
	} `json:"workflowExecutionLogs"`
}

// ---- Client methods ----

// ListExecutions fetches workflow executions matching the given filters.
// At most one page of results is returned; Limit controls page size (max 100).
func (c *Client) ListExecutions(parent context.Context, in ListExecutionsInput) ([]Execution, error) {
	ctx, cancel := c.CreateServiceContextWithTimeout(parent)
	defer cancel()

	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	input := map[string]any{
		"page": map[string]any{
			"number": 0,
			"size":   limit,
		},
	}
	if in.WorkflowUUID != nil {
		input["workflowUuid"] = *in.WorkflowUUID
	}
	if len(in.Statuses) > 0 {
		ss := make([]string, len(in.Statuses))
		for i, s := range in.Statuses {
			ss[i] = string(s)
		}
		input["status"] = ss
	}
	if in.From != nil {
		input["from"] = in.From.UTC().Format(time.RFC3339)
	}
	if in.To != nil {
		input["to"] = in.To.UTC().Format(time.RFC3339)
	}

	req := graphql.NewRequest(listExecutionsQuery)
	req.Var("input", input)

	var env listExecutionsEnvelope
	if err := c.graphql.Execute(ctx, req, &env); err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}

	return toExecutions(env.WorkflowExecutions.Data), nil
}

// FindExecutionByOnChainID resolves the platform UUID for an execution given its
// on-chain hex ID (the identifier shown in the Explorer UI). It searches recent
// executions and matches on the id field.
func (c *Client) FindExecutionByOnChainID(parent context.Context, onChainID string) (*Execution, error) {
	// The API has no direct filter by on-chain ID, so we fetch a broad page
	// and match client-side. The on-chain ID appears in recent executions.
	executions, err := c.ListExecutions(parent, ListExecutionsInput{Limit: 100})
	if err != nil {
		return nil, fmt.Errorf("searching for execution %q: %w", onChainID, err)
	}
	for _, e := range executions {
		if strings.EqualFold(e.ID, onChainID) {
			full, err := c.GetExecution(parent, e.UUID)
			if err != nil {
				return nil, err
			}
			return full, nil
		}
	}
	return nil, fmt.Errorf("execution with ID %q not found", onChainID)
}

// CountExecutions returns the total number of executions matching the given filters.
// It fetches only a single-item page — only the count field is used.
func (c *Client) CountExecutions(parent context.Context, workflowUUID string, statuses []ExecutionStatus) (int, error) {
	ctx, cancel := c.CreateServiceContextWithTimeout(parent)
	defer cancel()

	input := map[string]any{
		"workflowUuid": workflowUUID,
		"page":         map[string]any{"number": 0, "size": 1},
	}
	if len(statuses) > 0 {
		ss := make([]string, len(statuses))
		for i, s := range statuses {
			ss[i] = string(s)
		}
		input["status"] = ss
	}

	req := graphql.NewRequest(listExecutionsQuery)
	req.Var("input", input)

	var env listExecutionsEnvelope
	if err := c.graphql.Execute(ctx, req, &env); err != nil {
		return 0, fmt.Errorf("count executions: %w", err)
	}
	return env.WorkflowExecutions.Count, nil
}

// GetExecution fetches a single execution by its UUID.
func (c *Client) GetExecution(parent context.Context, uuid string) (*Execution, error) {
	ctx, cancel := c.CreateServiceContextWithTimeout(parent)
	defer cancel()

	req := graphql.NewRequest(getExecutionQuery)
	req.Var("input", map[string]any{"uuid": uuid})

	var env getExecutionEnvelope
	if err := c.graphql.Execute(ctx, req, &env); err != nil {
		return nil, fmt.Errorf("get execution: %w", err)
	}

	if env.WorkflowExecution.Data == nil {
		return nil, fmt.Errorf("execution %q not found", uuid)
	}

	e := toExecution(*env.WorkflowExecution.Data)
	return &e, nil
}

// ListExecutionEvents fetches all node/capability events for an execution.
func (c *Client) ListExecutionEvents(parent context.Context, in ListEventsInput) ([]ExecutionEvent, error) {
	ctx, cancel := c.CreateServiceContextWithTimeout(parent)
	defer cancel()

	input := map[string]any{
		"workflowExecutionUUID": in.ExecutionUUID,
	}
	if in.CapabilityID != nil {
		input["capabilityID"] = *in.CapabilityID
	}
	if in.Status != nil {
		input["status"] = *in.Status
	}

	req := graphql.NewRequest(listExecutionEventsQuery)
	req.Var("input", input)

	var env listEventsEnvelope
	if err := c.graphql.Execute(ctx, req, &env); err != nil {
		return nil, fmt.Errorf("list execution events: %w", err)
	}

	events := make([]ExecutionEvent, 0, len(env.WorkflowExecutionEvents.Data))
	for _, g := range env.WorkflowExecutionEvents.Data {
		errs := make([]CapabilityExecutionError, 0, len(g.Errors))
		for _, e := range g.Errors {
			errs = append(errs, CapabilityExecutionError(e))
		}
		events = append(events, ExecutionEvent{
			CapabilityID: g.CapabilityID,
			Status:       g.Status,
			StartedAt:    g.StartedAt,
			FinishedAt:   g.FinishedAt,
			Method:       g.Method,
			Errors:       errs,
		})
	}
	return events, nil
}

// ListExecutionLogs fetches all log lines for an execution.
func (c *Client) ListExecutionLogs(parent context.Context, executionUUID string) ([]ExecutionLog, error) {
	ctx, cancel := c.CreateServiceContextWithTimeout(parent)
	defer cancel()

	req := graphql.NewRequest(listExecutionLogsQuery)
	req.Var("input", map[string]any{"workflowExecutionUUID": executionUUID})

	var env listLogsEnvelope
	if err := c.graphql.Execute(ctx, req, &env); err != nil {
		return nil, fmt.Errorf("list execution logs: %w", err)
	}

	logs := make([]ExecutionLog, 0, len(env.WorkflowExecutionLogs.Data))
	for _, g := range env.WorkflowExecutionLogs.Data {
		logs = append(logs, ExecutionLog(g))
	}
	return logs, nil
}

// ---- helpers ----

func toExecution(g gqlExecution) Execution {
	errs := make([]ExecutionError, 0, len(g.Errors))
	for _, e := range g.Errors {
		errs = append(errs, ExecutionError(e))
	}
	return Execution{
		UUID:         g.UUID,
		ID:           g.ID,
		WorkflowUUID: g.WorkflowUUID,
		WorkflowID:   g.WorkflowID,
		WorkflowName: g.WorkflowName,
		Status:       ExecutionStatus(g.Status),
		StartedAt:    g.StartedAt,
		FinishedAt:   g.FinishedAt,
		CreditUsed:   g.CreditUsed,
		Errors:       errs,
	}
}

func toExecutions(gs []gqlExecution) []Execution {
	out := make([]Execution, 0, len(gs))
	for _, g := range gs {
		out = append(out, toExecution(g))
	}
	return out
}

package workflowdataclient

import (
	"context"
	"fmt"
	"time"

	"github.com/machinebox/graphql"
)

// WorkflowSummary is an extended workflow record including execution health fields.
type WorkflowSummary struct {
	UUID           string
	Name           string
	WorkflowID     string
	OwnerAddress   string
	Status         string
	WorkflowSource string
	RegisteredAt   time.Time
	ExecutedAt     *time.Time
	ExecutionCount int
	SuccessCount   int
	FailureCount   int
}

// WorkflowDeploymentRecord is a single deployment entry.
type WorkflowDeploymentRecord struct {
	UUID         string
	WorkflowID   string
	Status       string
	DeployedAt   time.Time
	TxHash       *string
	BinaryURL    *string
	ConfigURL    *string
	ErrorMessage *string
}

// ---- queries ----

const getWorkflowQuery = `
query GetWorkflow($input: WorkflowInput!) {
  workflow(input: $input) {
    data {
      uuid
      name
      workflowId
      ownerAddress
      status
      workflowSource
      registeredAt
      executedAt
      executionCount
      executionCountByStatus {
        success
        failure
      }
    }
  }
}
`

const getLatestDeploymentQuery = `
query GetLatestDeployment($input: WorkflowDeploymentsInput!) {
  workflowDeployments(input: $input) {
    data {
      uuid
      workflowID
      status
      deployedAt
      txHash
      binaryURL
      configURL
      errorMessage
    }
  }
}
`

// ---- envelopes ----

type gqlWorkflowSummary struct {
	UUID                   string     `json:"uuid"`
	Name                   string     `json:"name"`
	WorkflowID             string     `json:"workflowId"`
	OwnerAddress           string     `json:"ownerAddress"`
	Status                 string     `json:"status"`
	WorkflowSource         string     `json:"workflowSource"`
	RegisteredAt           time.Time  `json:"registeredAt"`
	ExecutedAt             *time.Time `json:"executedAt"`
	ExecutionCount         int        `json:"executionCount"`
	ExecutionCountByStatus struct {
		Success int `json:"success"`
		Failure int `json:"failure"`
	} `json:"executionCountByStatus"`
}

type getWorkflowEnvelope struct {
	Workflow struct {
		Data *gqlWorkflowSummary `json:"data"`
	} `json:"workflow"`
}

type gqlDeploymentRecord struct {
	UUID         string    `json:"uuid"`
	WorkflowID   string    `json:"workflowID"`
	Status       string    `json:"status"`
	DeployedAt   time.Time `json:"deployedAt"`
	TxHash       *string   `json:"txHash"`
	BinaryURL    *string   `json:"binaryURL"`
	ConfigURL    *string   `json:"configURL"`
	ErrorMessage *string   `json:"errorMessage"`
}

type getLatestDeploymentEnvelope struct {
	WorkflowDeployments struct {
		Data []gqlDeploymentRecord `json:"data"`
	} `json:"workflowDeployments"`
}

// ---- methods ----

// GetWorkflowSummary fetches extended workflow details including execution health.
func (c *Client) GetWorkflowSummary(parent context.Context, uuid string, from time.Time) (*WorkflowSummary, error) {
	ctx, cancel := c.CreateServiceContextWithTimeout(parent)
	defer cancel()

	req := graphql.NewRequest(getWorkflowQuery)
	req.Var("input", map[string]any{
		"uuid": uuid,
		"from": from.UTC().Format(time.RFC3339),
	})

	var env getWorkflowEnvelope
	if err := c.graphql.Execute(ctx, req, &env); err != nil {
		return nil, fmt.Errorf("get workflow summary: %w", err)
	}

	if env.Workflow.Data == nil {
		return nil, fmt.Errorf("workflow %q not found", uuid)
	}

	g := *env.Workflow.Data
	return &WorkflowSummary{
		UUID:           g.UUID,
		Name:           g.Name,
		WorkflowID:     g.WorkflowID,
		OwnerAddress:   g.OwnerAddress,
		Status:         g.Status,
		WorkflowSource: g.WorkflowSource,
		RegisteredAt:   g.RegisteredAt,
		ExecutedAt:     g.ExecutedAt,
		ExecutionCount: g.ExecutionCount,
		SuccessCount:   g.ExecutionCountByStatus.Success,
		FailureCount:   g.ExecutionCountByStatus.Failure,
	}, nil
}

// GetLatestDeployment fetches the most recent deployment record for a workflow.
// from/to mirror what the Explorer UI passes — the backend requires them even though
// the schema marks them optional.
func (c *Client) GetLatestDeployment(parent context.Context, workflowUUID string, from, to time.Time) (*WorkflowDeploymentRecord, error) {
	ctx, cancel := c.CreateServiceContextWithTimeout(parent)
	defer cancel()

	req := graphql.NewRequest(getLatestDeploymentQuery)
	req.Var("input", map[string]any{
		"workflowUUID": workflowUUID,
		"from":         from.UTC().Format(time.RFC3339),
		"to":           to.UTC().Format(time.RFC3339),
		"orderBy": map[string]any{
			"field": "DEPLOYED_AT",
			"order": "DESC",
		},
		"page": map[string]any{
			"number": 0,
			"size":   1,
		},
	})

	var env getLatestDeploymentEnvelope
	if err := c.graphql.Execute(ctx, req, &env); err != nil {
		return nil, fmt.Errorf("get latest deployment: %w", err)
	}

	if len(env.WorkflowDeployments.Data) == 0 {
		return nil, nil //nolint:nilnil // no deployment record is a valid state
	}

	g := env.WorkflowDeployments.Data[0]
	return &WorkflowDeploymentRecord{
		UUID:         g.UUID,
		WorkflowID:   g.WorkflowID,
		Status:       g.Status,
		DeployedAt:   g.DeployedAt,
		TxHash:       g.TxHash,
		BinaryURL:    g.BinaryURL,
		ConfigURL:    g.ConfigURL,
		ErrorMessage: g.ErrorMessage,
	}, nil
}

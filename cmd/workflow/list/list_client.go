package list

import (
	"context"
	"fmt"

	"github.com/machinebox/graphql"
)

const DefaultPageSize = 100

// Workflow is a workflow row from the platform list API, decoupled from transport JSON.
type Workflow struct {
	Name           string
	WorkflowID     string
	OwnerAddress   string
	Status         string
	WorkflowSource string
}

const listWorkflowsQuery = `
query ListWorkflows($input: WorkflowsInput!) {
  workflows(input: $input) {
    data {
      name
      workflowId
      ownerAddress
      status
      workflowSource
    }
    count
  }
}
`

// Executor runs a GraphQL request (e.g. graphqlclient.Client).
type Executor interface {
	Execute(ctx context.Context, req *graphql.Request, resp any) error
}

type gqlWorkflow struct {
	Name           string `json:"name"`
	WorkflowID     string `json:"workflowId"`
	OwnerAddress   string `json:"ownerAddress"`
	Status         string `json:"status"`
	WorkflowSource string `json:"workflowSource"`
}

type listWorkflowsEnvelope struct {
	Workflows struct {
		Data  []gqlWorkflow `json:"data"`
		Count int           `json:"count"`
	} `json:"workflows"`
}

// ListAll pages through ListWorkflows and returns the aggregated workflows.
func ListAll(ctx context.Context, exec Executor, pageSize int) ([]Workflow, error) {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	var total int
	all := make([]Workflow, 0)

	for pageNum := 0; ; pageNum++ {
		req := graphql.NewRequest(listWorkflowsQuery)
		req.Var("input", map[string]any{
			"page": map[string]any{
				"number": pageNum,
				"size":   pageSize,
			},
		})

		var env listWorkflowsEnvelope
		if err := exec.Execute(ctx, req, &env); err != nil {
			return nil, fmt.Errorf("list workflows: %w", err)
		}

		if pageNum == 0 {
			total = env.Workflows.Count
		}

		batch := env.Workflows.Data
		for _, g := range batch {
			all = append(all, Workflow{
				Name:           g.Name,
				WorkflowID:     g.WorkflowID,
				OwnerAddress:   g.OwnerAddress,
				Status:         g.Status,
				WorkflowSource: g.WorkflowSource,
			})
		}

		if len(all) >= total || len(batch) == 0 {
			break
		}
	}

	return all, nil
}

package workflowdataclient

import (
	"context"
	"fmt"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
)

const DefaultPageSize = 100

// Workflow is a workflow row returned by the platform list API.
type Workflow struct {
	Name           string
	WorkflowID     string
	OwnerAddress   string
	Status         string
	WorkflowSource string
}

// Client fetches workflow data from the CRE platform GraphQL API.
type Client struct {
	graphql *graphqlclient.Client
	log     *zerolog.Logger
}

// New creates a WorkflowDataClient backed by the provided GraphQL client.
func New(gql *graphqlclient.Client, log *zerolog.Logger) *Client {
	return &Client{graphql: gql, log: log}
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

// ListAll pages through the ListWorkflows query and returns all workflows.
func (c *Client) ListAll(ctx context.Context, pageSize int) ([]Workflow, error) {
	return c.list(ctx, pageSize, "")
}

// SearchByName pages through the ListWorkflows query with the given search
// filter (server-side contains match on workflow name).
func (c *Client) SearchByName(ctx context.Context, name string, pageSize int) ([]Workflow, error) {
	return c.list(ctx, pageSize, name)
}

func (c *Client) list(ctx context.Context, pageSize int, search string) ([]Workflow, error) {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}

	var total int
	all := make([]Workflow, 0)

	for pageNum := 0; ; pageNum++ {
		req := graphql.NewRequest(listWorkflowsQuery)
		input := map[string]any{
			"page": map[string]any{
				"number": pageNum,
				"size":   pageSize,
			},
		}
		if search != "" {
			input["search"] = search
		}
		req.Var("input", input)

		var env listWorkflowsEnvelope
		if err := c.graphql.Execute(ctx, req, &env); err != nil {
			return nil, fmt.Errorf("list workflows: %w", err)
		}

		if pageNum == 0 {
			total = env.Workflows.Count
		}

		batch := env.Workflows.Data
		for _, g := range batch {
			all = append(all, Workflow(g))
		}

		if len(all) >= total || len(batch) == 0 {
			break
		}
	}

	c.log.Debug().Int("count", len(all)).Str("search", search).Msg("Listed workflows from platform")
	return all, nil
}

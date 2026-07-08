package workflowdataclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
)

const DefaultPageSize = 100

// Workflow is a workflow row returned by the platform list API.
type Workflow struct {
	UUID           string
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
	timeout time.Duration
}

// New creates a WorkflowDataClient backed by the provided GraphQL client.
func New(gql *graphqlclient.Client, log *zerolog.Logger) *Client {
	return &Client{graphql: gql, log: log, timeout: time.Minute}
}

func (c *Client) SetServiceTimeout(timeout time.Duration) {
	c.timeout = timeout
}

func (c *Client) CreateServiceContextWithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, c.timeout) //nolint:gosec // G118 -- cancel is deferred by callers
}

const listWorkflowsQuery = `
query ListWorkflows($input: WorkflowsInput!) {
  workflows(input: $input) {
    data {
      uuid
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
	UUID           string `json:"uuid"`
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

// ListFilter controls optional server-side filters for ListWorkflows.
type ListFilter struct {
	Search               string
	WorkflowOwnerAddress string
}

// ListAll pages through the ListWorkflows query and returns all workflows.
func (c *Client) ListAll(parent context.Context, pageSize int) ([]Workflow, error) {
	return c.ListWithFilter(parent, pageSize, ListFilter{})
}

// SearchByName pages through the ListWorkflows query with the given search
// filter (server-side contains match on workflow name). When ownerAddress is
// non-empty, results are scoped to that workflow owner.
func (c *Client) SearchByName(parent context.Context, name string, pageSize int, ownerAddress string) ([]Workflow, error) {
	return c.ListWithFilter(parent, pageSize, ListFilter{
		Search:               name,
		WorkflowOwnerAddress: ownerAddress,
	})
}

// ListWithFilter pages through the ListWorkflows query with optional search
// and workflow-owner filters.
func (c *Client) ListWithFilter(parent context.Context, pageSize int, filter ListFilter) ([]Workflow, error) {
	return c.list(parent, pageSize, filter)
}

func (c *Client) list(parent context.Context, pageSize int, filter ListFilter) ([]Workflow, error) {
	ctx, cancel := c.CreateServiceContextWithTimeout(parent)
	defer cancel()
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
		if filter.Search != "" {
			input["search"] = filter.Search
		}
		if owner := strings.TrimSpace(filter.WorkflowOwnerAddress); owner != "" {
			input["workflowOwnerAddress"] = []string{owner}
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

	c.log.Debug().
		Int("count", len(all)).
		Str("search", filter.Search).
		Str("workflowOwnerAddress", filter.WorkflowOwnerAddress).
		Msg("Listed workflows from platform")
	return all, nil
}

package privateregistryclient

import (
	"context"
	"fmt"
	"time"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
)

type Client struct {
	graphql        *graphqlclient.Client
	log            *zerolog.Logger
	serviceTimeout time.Duration
}

func New(gql *graphqlclient.Client, log *zerolog.Logger) *Client {
	return &Client{
		graphql:        gql,
		log:            log,
		serviceTimeout: 2 * time.Minute,
	}
}

func (c *Client) SetServiceTimeout(timeout time.Duration) {
	c.serviceTimeout = timeout
}

func (c *Client) CreateServiceContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.serviceTimeout) //nolint:gosec // G118 -- cancel is deferred by callers
}

type WorkflowInRegistry struct {
	WorkflowID     string                   `json:"workflowId"`
	Owner          string                   `json:"owner"`
	CreatedAt      string                   `json:"createdAt"`
	Status         WorkflowInRegistryStatus `json:"status"`
	WorkflowName   string                   `json:"workflowName"`
	BinaryURL      string                   `json:"binaryUrl"`
	ConfigURL      string                   `json:"configUrl"`
	Tag            string                   `json:"tag"`
	Attributes     string                   `json:"attributes"`
	DonFamily      string                   `json:"donFamily"`
	OrganizationID string                   `json:"organizationId"`
}

type WorkflowInRegistryStatus string

const (
	WorkflowStatusUnspecified WorkflowInRegistryStatus = "WORKFLOW_STATUS_UNSPECIFIED"
	WorkflowStatusActive      WorkflowInRegistryStatus = "WORKFLOW_STATUS_ACTIVE"
	WorkflowStatusPaused      WorkflowInRegistryStatus = "WORKFLOW_STATUS_PAUSED"
)

const (
	maxWorkflowNameLength = 64
	maxBinaryURLLength    = 200
	maxConfigURLLength    = 200
	maxTagLength          = 32
	maxAttributesLength   = 1024
)

type WorkflowInRegistryInput struct {
	WorkflowID   string                   `json:"workflowId"`
	Status       WorkflowInRegistryStatus `json:"status"`
	WorkflowName string                   `json:"workflowName"`
	BinaryURL    string                   `json:"binaryUrl"`
	ConfigURL    *string                  `json:"configUrl,omitempty"`
	Tag          *string                  `json:"tag,omitempty"`
	Attributes   *string                  `json:"attributes,omitempty"`
	DonFamily    string                   `json:"donFamily"`
}

type UpsertWorkflowInRegistryRequest struct {
	Workflow WorkflowInRegistryInput `json:"workflow"`
}

type UpsertWorkflowInRegistryResponse struct {
	Workflow WorkflowInRegistry `json:"workflow"`
}

type PauseWorkflowInRegistryRequest struct {
	WorkflowID string `json:"workflowId"`
}

type PauseWorkflowInRegistryResponse struct {
	Workflow WorkflowInRegistry `json:"workflow"`
}

type ActivateWorkflowInRegistryRequest struct {
	WorkflowID string `json:"workflowId"`
}

type ActivateWorkflowInRegistryResponse struct {
	Workflow WorkflowInRegistry `json:"workflow"`
}

type DeleteWorkflowInRegistryRequest struct {
	WorkflowID string `json:"workflowId"`
}

type DeleteWorkflowInRegistryResponse struct {
	WorkflowID string `json:"workflowId"`
}

func (c *Client) UpsertWorkflowInRegistry(workflow WorkflowInRegistryInput) (WorkflowInRegistry, error) {
	if err := validateUpsertWorkflowInput(workflow); err != nil {
		return WorkflowInRegistry{}, err
	}

	const mutation = `
mutation UpsertWorkflowInRegistry($request: UpsertWorkflowInRegistryRequest!) {
  upsertWorkflowInRegistry(request: $request) {
    workflow {
      workflowId
      owner
      createdAt
      status
      workflowName
      binaryUrl
      configUrl
      tag
      attributes
      donFamily
      organizationId
    }
  }
}`

	req := graphql.NewRequest(mutation)
	req.Var("request", UpsertWorkflowInRegistryRequest{Workflow: workflow})

	var container struct {
		UpsertWorkflowInRegistry UpsertWorkflowInRegistryResponse `json:"upsertWorkflowInRegistry"`
	}

	ctx, cancel := c.CreateServiceContextWithTimeout()
	defer cancel()

	if err := c.graphql.Execute(ctx, req, &container); err != nil {
		return WorkflowInRegistry{}, fmt.Errorf("upsert workflow in registry: %w", err)
	}

	c.log.Debug().Str("workflowId", container.UpsertWorkflowInRegistry.Workflow.WorkflowID).
		Msg("Upserted workflow in private registry")

	return container.UpsertWorkflowInRegistry.Workflow, nil
}

func (c *Client) PauseWorkflowInRegistry(workflowID string) (WorkflowInRegistry, error) {
	if workflowID == "" {
		return WorkflowInRegistry{}, fmt.Errorf("workflowId is required")
	}

	const mutation = `
mutation PauseWorkflowInRegistry($request: PauseWorkflowInRegistryRequest!) {
  pauseWorkflowInRegistry(request: $request) {
    workflow {
      workflowId
      owner
      createdAt
      status
      workflowName
      binaryUrl
      configUrl
      tag
      attributes
      donFamily
      organizationId
    }
  }
}`

	req := graphql.NewRequest(mutation)
	req.Var("request", PauseWorkflowInRegistryRequest{WorkflowID: workflowID})

	var container struct {
		PauseWorkflowInRegistry PauseWorkflowInRegistryResponse `json:"pauseWorkflowInRegistry"`
	}

	ctx, cancel := c.CreateServiceContextWithTimeout()
	defer cancel()

	if err := c.graphql.Execute(ctx, req, &container); err != nil {
		return WorkflowInRegistry{}, fmt.Errorf("pause workflow in registry: %w", err)
	}

	c.log.Debug().Str("workflowId", workflowID).
		Msg("Paused workflow in private registry")

	return container.PauseWorkflowInRegistry.Workflow, nil
}

func (c *Client) ActivateWorkflowInRegistry(workflowID string) (WorkflowInRegistry, error) {
	if workflowID == "" {
		return WorkflowInRegistry{}, fmt.Errorf("workflowId is required")
	}

	const mutation = `
mutation ActivateWorkflowInRegistry($request: ActivateWorkflowInRegistryRequest!) {
  activateWorkflowInRegistry(request: $request) {
    workflow {
      workflowId
      owner
      createdAt
      status
      workflowName
      binaryUrl
      configUrl
      tag
      attributes
      donFamily
      organizationId
    }
  }
}`

	req := graphql.NewRequest(mutation)
	req.Var("request", ActivateWorkflowInRegistryRequest{WorkflowID: workflowID})

	var container struct {
		ActivateWorkflowInRegistry ActivateWorkflowInRegistryResponse `json:"activateWorkflowInRegistry"`
	}

	ctx, cancel := c.CreateServiceContextWithTimeout()
	defer cancel()

	if err := c.graphql.Execute(ctx, req, &container); err != nil {
		return WorkflowInRegistry{}, fmt.Errorf("activate workflow in registry: %w", err)
	}

	c.log.Debug().Str("workflowId", workflowID).
		Msg("Activated workflow in private registry")

	return container.ActivateWorkflowInRegistry.Workflow, nil
}

func (c *Client) DeleteWorkflowInRegistry(workflowID string) (string, error) {
	if workflowID == "" {
		return "", fmt.Errorf("workflowId is required")
	}

	const mutation = `
mutation DeleteWorkflowInRegistry($request: DeleteWorkflowInRegistryRequest!) {
  deleteWorkflowInRegistry(request: $request) {
    workflowId
  }
}`

	req := graphql.NewRequest(mutation)
	req.Var("request", DeleteWorkflowInRegistryRequest{WorkflowID: workflowID})

	var container struct {
		DeleteWorkflowInRegistry DeleteWorkflowInRegistryResponse `json:"deleteWorkflowInRegistry"`
	}

	ctx, cancel := c.CreateServiceContextWithTimeout()
	defer cancel()

	if err := c.graphql.Execute(ctx, req, &container); err != nil {
		return "", fmt.Errorf("delete workflow in registry: %w", err)
	}

	c.log.Debug().Str("workflowId", workflowID).
		Msg("Deleted workflow in private registry")

	return container.DeleteWorkflowInRegistry.WorkflowID, nil
}

func validateUpsertWorkflowInput(input WorkflowInRegistryInput) error {
	if input.WorkflowID == "" {
		return fmt.Errorf("workflowId is required")
	}
	if input.Status == "" {
		return fmt.Errorf("status is required")
	}
	if input.Status != WorkflowStatusUnspecified &&
		input.Status != WorkflowStatusActive &&
		input.Status != WorkflowStatusPaused {
		return fmt.Errorf("status must be one of %q, %q, %q", WorkflowStatusUnspecified, WorkflowStatusActive, WorkflowStatusPaused)
	}
	if input.WorkflowName == "" {
		return fmt.Errorf("workflowName is required")
	}
	if len(input.WorkflowName) > maxWorkflowNameLength {
		return fmt.Errorf("workflowName exceeds max length %d", maxWorkflowNameLength)
	}
	if input.BinaryURL == "" {
		return fmt.Errorf("binaryUrl is required")
	}
	if len(input.BinaryURL) > maxBinaryURLLength {
		return fmt.Errorf("binaryUrl exceeds max length %d", maxBinaryURLLength)
	}
	if err := validateOptionalLength("configUrl", input.ConfigURL, maxConfigURLLength); err != nil {
		return err
	}
	if err := validateOptionalLength("tag", input.Tag, maxTagLength); err != nil {
		return err
	}
	if err := validateOptionalLength("attributes", input.Attributes, maxAttributesLength); err != nil {
		return err
	}
	if input.DonFamily == "" {
		return fmt.Errorf("donFamily is required")
	}

	return nil
}

func validateOptionalLength(name string, value *string, maxLen int) error {
	if value != nil && len(*value) > maxLen {
		return fmt.Errorf("%s exceeds max length %d", name, maxLen)
	}

	return nil
}

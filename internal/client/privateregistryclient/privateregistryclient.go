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

type OffchainWorkflow struct {
	WorkflowID     string                 `json:"workflowId"`
	Owner          string                 `json:"owner"`
	CreatedAt      string                 `json:"createdAt"`
	Status         OffchainWorkflowStatus `json:"status"`
	WorkflowName   string                 `json:"workflowName"`
	BinaryURL      string                 `json:"binaryUrl"`
	ConfigURL      string                 `json:"configUrl"`
	Tag            string                 `json:"tag"`
	Attributes     string                 `json:"attributes"`
	DonFamily      string                 `json:"donFamily"`
	OrganizationID string                 `json:"organizationId"`
}

type OffchainWorkflowStatus string

const (
	WorkflowStatusUnspecified OffchainWorkflowStatus = "WORKFLOW_STATUS_UNSPECIFIED"
	WorkflowStatusActive      OffchainWorkflowStatus = "WORKFLOW_STATUS_ACTIVE"
	WorkflowStatusPaused      OffchainWorkflowStatus = "WORKFLOW_STATUS_PAUSED"
)

const (
	maxWorkflowNameLength = 64
	maxBinaryURLLength    = 200
	maxConfigURLLength    = 200
	maxTagLength          = 32
	maxAttributesLength   = 1024
)

type OffchainWorkflowInput struct {
	WorkflowID   string                 `json:"workflowId"`
	Status       OffchainWorkflowStatus `json:"status"`
	WorkflowName string                 `json:"workflowName"`
	BinaryURL    string                 `json:"binaryUrl"`
	ConfigURL    *string                `json:"configUrl,omitempty"`
	Tag          *string                `json:"tag,omitempty"`
	Attributes   *string                `json:"attributes,omitempty"`
	DonFamily    string                 `json:"donFamily"`
}

type UpsertOffchainWorkflowRequest struct {
	Workflow OffchainWorkflowInput `json:"workflow"`
}

type UpsertOffchainWorkflowResponse struct {
	Workflow OffchainWorkflow `json:"workflow"`
}

type PauseOffchainWorkflowRequest struct {
	WorkflowID string `json:"workflowId"`
}

type PauseOffchainWorkflowResponse struct {
	Workflow OffchainWorkflow `json:"workflow"`
}

type ActivateOffchainWorkflowRequest struct {
	WorkflowID string `json:"workflowId"`
}

type ActivateOffchainWorkflowResponse struct {
	Workflow OffchainWorkflow `json:"workflow"`
}

type DeleteOffchainWorkflowRequest struct {
	WorkflowID string `json:"workflowId"`
}

type DeleteOffchainWorkflowResponse struct {
	WorkflowID string `json:"workflowId"`
}

func (c *Client) UpsertWorkflowInRegistry(workflow OffchainWorkflowInput) (OffchainWorkflow, error) {
	if err := validateUpsertWorkflowInput(workflow); err != nil {
		return OffchainWorkflow{}, err
	}

	const mutation = `
mutation UpsertOffchainWorkflow($request: UpsertOffchainWorkflowRequest!) {
  upsertOffchainWorkflow(request: $request) {
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
	req.Var("request", UpsertOffchainWorkflowRequest{Workflow: workflow})

	var container struct {
		UpsertOffchainWorkflow UpsertOffchainWorkflowResponse `json:"upsertOffchainWorkflow"`
	}

	ctx, cancel := c.CreateServiceContextWithTimeout()
	defer cancel()

	if err := c.graphql.Execute(ctx, req, &container); err != nil {
		return OffchainWorkflow{}, fmt.Errorf("upsert workflow in registry: %w", err)
	}

	c.log.Debug().Str("workflowId", container.UpsertOffchainWorkflow.Workflow.WorkflowID).
		Msg("Upserted workflow in private registry")

	return container.UpsertOffchainWorkflow.Workflow, nil
}

func (c *Client) PauseWorkflowInRegistry(workflowID string) (OffchainWorkflow, error) {
	if workflowID == "" {
		return OffchainWorkflow{}, fmt.Errorf("workflowId is required")
	}

	const mutation = `
mutation PauseOffchainWorkflow($request: PauseOffchainWorkflowRequest!) {
  pauseOffchainWorkflow(request: $request) {
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
	req.Var("request", PauseOffchainWorkflowRequest{WorkflowID: workflowID})

	var container struct {
		PauseOffchainWorkflow PauseOffchainWorkflowResponse `json:"pauseOffchainWorkflow"`
	}

	ctx, cancel := c.CreateServiceContextWithTimeout()
	defer cancel()

	if err := c.graphql.Execute(ctx, req, &container); err != nil {
		return OffchainWorkflow{}, fmt.Errorf("pause workflow in registry: %w", err)
	}

	c.log.Debug().Str("workflowId", workflowID).
		Msg("Paused workflow in private registry")

	return container.PauseOffchainWorkflow.Workflow, nil
}

func (c *Client) ActivateWorkflowInRegistry(workflowID string) (OffchainWorkflow, error) {
	if workflowID == "" {
		return OffchainWorkflow{}, fmt.Errorf("workflowId is required")
	}

	const mutation = `
mutation ActivateOffchainWorkflow($request: ActivateOffchainWorkflowRequest!) {
  activateOffchainWorkflow(request: $request) {
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
	req.Var("request", ActivateOffchainWorkflowRequest{WorkflowID: workflowID})

	var container struct {
		ActivateOffchainWorkflow ActivateOffchainWorkflowResponse `json:"activateOffchainWorkflow"`
	}

	ctx, cancel := c.CreateServiceContextWithTimeout()
	defer cancel()

	if err := c.graphql.Execute(ctx, req, &container); err != nil {
		return OffchainWorkflow{}, fmt.Errorf("activate workflow in registry: %w", err)
	}

	c.log.Debug().Str("workflowId", workflowID).
		Msg("Activated workflow in private registry")

	return container.ActivateOffchainWorkflow.Workflow, nil
}

func (c *Client) DeleteWorkflowInRegistry(workflowID string) (string, error) {
	if workflowID == "" {
		return "", fmt.Errorf("workflowId is required")
	}

	const mutation = `
mutation DeleteOffchainWorkflow($request: DeleteOffchainWorkflowRequest!) {
  deleteOffchainWorkflow(request: $request) {
    workflowId
  }
}`

	req := graphql.NewRequest(mutation)
	req.Var("request", DeleteOffchainWorkflowRequest{WorkflowID: workflowID})

	var container struct {
		DeleteOffchainWorkflow DeleteOffchainWorkflowResponse `json:"deleteOffchainWorkflow"`
	}

	ctx, cancel := c.CreateServiceContextWithTimeout()
	defer cancel()

	if err := c.graphql.Execute(ctx, req, &container); err != nil {
		return "", fmt.Errorf("delete workflow in registry: %w", err)
	}

	c.log.Debug().Str("workflowId", workflowID).
		Msg("Deleted workflow in private registry")

	return container.DeleteOffchainWorkflow.WorkflowID, nil
}

func validateUpsertWorkflowInput(input OffchainWorkflowInput) error {
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

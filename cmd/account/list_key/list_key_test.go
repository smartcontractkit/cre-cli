package list_key

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

type mockGraphQLClient struct {
	response struct {
		ListWorkflowOwners struct {
			LinkedOwners []WorkflowOwner `json:"linkedOwners"`
		} `json:"listWorkflowOwners"`
	}
	err error
}

func NewTestHandler(log *zerolog.Logger, client GraphQLExecutor) *Handler {
	return &Handler{
		log:            log,
		client:         client,
		credentials:    &credentials.Credentials{},
		environmentSet: &environments.EnvironmentSet{},
	}
}

func (m *mockGraphQLClient) Execute(_ context.Context, _ *graphql.Request, resp any) error {
	if m.err != nil {
		return m.err
	}
	out, ok := resp.(*struct {
		ListWorkflowOwners struct {
			LinkedOwners []WorkflowOwner `json:"linkedOwners"`
		} `json:"listWorkflowOwners"`
	})
	if !ok {
		return errors.New("invalid response envelope")
	}
	*out = m.response
	return nil
}

func TestExecute_Success(t *testing.T) {
	logOutput := new(strings.Builder)
	logger := zerolog.New(logOutput)

	mockResp := mockGraphQLClient{
		response: struct {
			ListWorkflowOwners struct {
				LinkedOwners []WorkflowOwner `json:"linkedOwners"`
			} `json:"listWorkflowOwners"`
		}{
			ListWorkflowOwners: struct {
				LinkedOwners []WorkflowOwner `json:"linkedOwners"`
			}{
				LinkedOwners: []WorkflowOwner{
					{
						WorkflowOwnerLabel:   "Alice",
						WorkflowOwnerAddress: "0xabc",
						VerificationStatus:   "VERIFIED",
						VerifiedAt:           "2025-07-30",
						ChainSelector:        "111",
						ContractAddress:      "0xcontract1",
					},
				},
			},
		},
	}

	h := NewTestHandler(&logger, &mockResp)

	err := h.Execute(context.Background())
	require.NoError(t, err)

	output := logOutput.String()
	assert.Contains(t, output, "Workflow owners retrieved successfully")
	assert.Contains(t, output, "1. Alice")
}

func TestExecute_Error(t *testing.T) {
	logOutput := new(strings.Builder)
	logger := zerolog.New(logOutput)

	mockErrClient := &mockGraphQLClient{
		err: errors.New("network error"),
	}

	h := NewTestHandler(&logger, mockErrClient)

	err := h.Execute(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch workflow owners failed")
}

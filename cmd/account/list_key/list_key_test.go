package list_key

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
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
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

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

	h := NewTestHandler(nil, &mockResp)

	err := h.Execute(context.Background(), Inputs{})
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	var output strings.Builder
	_, _ = io.Copy(&output, r)

	assert.Contains(t, output.String(), "Workflow owners retrieved successfully")
	assert.Contains(t, output.String(), "1. Alice")
}

func TestExecute_Error(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	mockErrClient := &mockGraphQLClient{
		err: errors.New("network error"),
	}

	h := NewTestHandler(nil, mockErrClient)

	err := h.Execute(context.Background(), Inputs{})
	w.Close()
	os.Stdout = oldStdout
	var output strings.Builder
	_, _ = io.Copy(&output, r)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch workflow owners failed")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func mockWithOwners(owners ...WorkflowOwner) *mockGraphQLClient {
	m := &mockGraphQLClient{}
	m.response.ListWorkflowOwners.LinkedOwners = owners
	return m
}

func TestExecute_JSONOutput(t *testing.T) {
	h := NewTestHandler(nil, mockWithOwners(
		WorkflowOwner{
			WorkflowOwnerAddress: "0xabc",
			WorkflowOwnerLabel:   "Alice",
			Environment:          "PRODUCTION",
			VerificationStatus:   "VERIFIED",
			VerifiedAt:           "2025-07-30",
			ChainSelector:        "111",
			ContractAddress:      "0xcontract1",
		},
		WorkflowOwner{WorkflowOwnerAddress: "0xdef", WorkflowOwnerLabel: "Bob"},
	))

	var err error
	out := captureStdout(t, func() { err = h.Execute(context.Background(), Inputs{OutputFormat: "json"}) })
	require.NoError(t, err)

	var got []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got), "stdout must be only JSON, got %q", out)
	require.Len(t, got, 2)
	assert.Equal(t, map[string]any{
		"workflowOwnerAddress": "0xabc",
		"workflowOwnerLabel":   "Alice",
		"environment":          "PRODUCTION",
		"verificationStatus":   "VERIFIED",
		"verifiedAt":           "2025-07-30",
	}, got[0])
	assert.Equal(t, "0xdef", got[1]["workflowOwnerAddress"])
}

func TestExecute_JSONOutput_EmptyIsArray(t *testing.T) {
	h := NewTestHandler(nil, mockWithOwners())

	var err error
	out := captureStdout(t, func() { err = h.Execute(context.Background(), Inputs{OutputFormat: "json"}) })
	require.NoError(t, err)
	assert.Equal(t, "[]", strings.TrimSpace(out))
}

func TestExecute_JSONOutput_ErrorPrintsNothing(t *testing.T) {
	h := NewTestHandler(nil, &mockGraphQLClient{err: errors.New("network error")})

	var err error
	out := captureStdout(t, func() { err = h.Execute(context.Background(), Inputs{OutputFormat: "json"}) })
	require.Error(t, err)
	assert.Empty(t, out)
}

func TestResolveInputs(t *testing.T) {
	in, err := resolveInputs("", true)
	require.NoError(t, err)
	assert.Equal(t, "json", in.OutputFormat)

	in, err = resolveInputs("", false)
	require.NoError(t, err)
	assert.Empty(t, in.OutputFormat)

	_, err = resolveInputs("csv", false)
	require.Error(t, err)
}

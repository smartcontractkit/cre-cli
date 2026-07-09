package workflowresolve_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

type stubWorkflowLookup struct {
	listAll      []workflowdataclient.Workflow
	searchByName []workflowdataclient.Workflow
}

func (s stubWorkflowLookup) ListAll(_ context.Context, _ int) ([]workflowdataclient.Workflow, error) {
	return s.listAll, nil
}

func (s stubWorkflowLookup) SearchByName(_ context.Context, _ string, _ int) ([]workflowdataclient.Workflow, error) {
	return s.searchByName, nil
}

func TestResolveWorkflowUUID_ByName_SingleActive(t *testing.T) {
	t.Parallel()
	wdc := stubWorkflowLookup{
		searchByName: []workflowdataclient.Workflow{
			{UUID: "wf-1", Name: "my-wf", Status: "ACTIVE"},
		},
	}
	uuid, err := workflowresolve.ResolveWorkflowUUID(context.Background(), wdc, "my-wf", workflowresolve.ResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, "wf-1", uuid)
}

func TestResolveWorkflowUUID_ByName_MultipleActive(t *testing.T) {
	t.Parallel()
	wdc := stubWorkflowLookup{
		searchByName: []workflowdataclient.Workflow{
			{UUID: "wf-1", Name: "my-wf", Status: "ACTIVE"},
			{UUID: "wf-2", Name: "my-wf", Status: "ACTIVE"},
		},
	}
	_, err := workflowresolve.ResolveWorkflowUUID(context.Background(), wdc, "my-wf", workflowresolve.ResolveOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple ACTIVE")
}

func TestResolveWorkflowUUID_ByWorkflowID(t *testing.T) {
	t.Parallel()
	onChainID := "00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856"
	wdc := stubWorkflowLookup{
		listAll: []workflowdataclient.Workflow{
			{UUID: "wf-1", WorkflowID: onChainID, Name: "my-wf"},
		},
	}
	uuid, err := workflowresolve.ResolveWorkflowUUID(context.Background(), wdc, onChainID, workflowresolve.ResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, "wf-1", uuid)
}

func TestLooksLikeWorkflowID(t *testing.T) {
	t.Parallel()
	assert.True(t, workflowresolve.LooksLikeWorkflowID("00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856"))
	assert.False(t, workflowresolve.LooksLikeWorkflowID("short"))
	assert.False(t, workflowresolve.LooksLikeWorkflowID("00000000-0000-0000-0000-000000000001"))
}

func TestResolveOutputFormat(t *testing.T) {
	t.Parallel()
	out, err := workflowresolve.ResolveOutputFormat("", true)
	require.NoError(t, err)
	assert.Equal(t, workflowresolve.OutputFormatJSON, out)

	_, err = workflowresolve.ResolveOutputFormat("csv", false)
	require.Error(t, err)
}

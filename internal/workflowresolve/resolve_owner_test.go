package workflowresolve_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/settings"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

func TestResolveWorkflowOwnerAddress_OnChain(t *testing.T) {
	t.Parallel()
	s := &settings.Settings{}
	s.Workflow.UserWorkflowSettings.WorkflowOwnerAddress = "0xabc"

	owner, err := workflowresolve.ResolveWorkflowOwnerAddress(s, nil, "")
	require.NoError(t, err)
	assert.Equal(t, "0xabc", owner)
}

func TestResolveWorkflowOwnerAddress_OffChainDerived(t *testing.T) {
	t.Parallel()
	s := &settings.Settings{}
	registry := &settings.OffChainRegistry{}

	owner, err := workflowresolve.ResolveWorkflowOwnerAddress(s, registry, "0xderived")
	require.NoError(t, err)
	assert.Equal(t, "0xderived", owner)
}

func TestResolveWorkflowOwnerAddress_OffChainMissingDerived(t *testing.T) {
	t.Parallel()
	s := &settings.Settings{}
	registry := &settings.OffChainRegistry{}

	_, err := workflowresolve.ResolveWorkflowOwnerAddress(s, registry, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "derived workflow owner")
}

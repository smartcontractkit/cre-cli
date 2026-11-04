package workflowresolve

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/client/workflowdataclient"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
)

type fakeLookup struct {
	all       []Workflow
	listErr   error
	searchErr error

	lastSearchName  string
	lastSearchOwner string
	searchCalls     int
	listCalls       int
}

func (f *fakeLookup) ListAll(ctx context.Context, pageSize int) ([]workflowdataclient.Workflow, error) {
	f.listCalls++
	return f.all, f.listErr
}

func (f *fakeLookup) SearchByName(ctx context.Context, name string, pageSize int, ownerAddress string) ([]workflowdataclient.Workflow, error) {
	f.searchCalls++
	f.lastSearchName = name
	f.lastSearchOwner = ownerAddress
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	// Mimic the server-side contains match.
	var out []workflowdataclient.Workflow
	for _, w := range f.all {
		if strings.Contains(strings.ToLower(w.Name), strings.ToLower(name)) {
			out = append(out, w)
		}
	}
	return out, nil
}

const (
	testWorkflowID1 = "00da21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856"
	testWorkflowID2 = "11eb32c9c4f228f42a4b4f9b1896336dcfe7d11394b954282707a70203c89667"
)

func strptr(s string) *string { return &s }

func testRegistries() []*tenantctx.Registry {
	return []*tenantctx.Registry{
		{ID: "private", Type: "off-chain"},
		{ID: "onchain-sepolia", Type: "on-chain", Address: strptr("0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D"), ChainSelector: strptr("16015286601757825753")},
	}
}

func TestResolveWorkflowRow_ByID(t *testing.T) {
	t.Parallel()

	lookup := &fakeLookup{all: []Workflow{
		{UUID: "u1", Name: "wf-one", WorkflowID: testWorkflowID1, OwnerAddress: "0xabc", Status: "ACTIVE", WorkflowSource: "private"},
		{UUID: "u2", Name: "wf-two", WorkflowID: testWorkflowID2, OwnerAddress: "0xdef", Status: "PAUSED", WorkflowSource: "private"},
	}}

	row, err := ResolveWorkflowRow(context.Background(), lookup, testWorkflowID1, RowResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, "u1", row.UUID)

	// Case-insensitive ID match.
	row, err = ResolveWorkflowRow(context.Background(), lookup, strings.ToUpper(testWorkflowID2), RowResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, "u2", row.UUID)

	_, err = ResolveWorkflowRow(context.Background(), lookup, "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", RowResolveOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no workflow found with ID")
}

func TestResolveWorkflowRow_ByID_Ineligible(t *testing.T) {
	t.Parallel()

	lookup := &fakeLookup{all: []Workflow{
		{UUID: "u2", Name: "wf-two", WorkflowID: testWorkflowID2, Status: "PAUSED", WorkflowSource: "private"},
	}}

	_, err := ResolveWorkflowRow(context.Background(), lookup, testWorkflowID2, RowResolveOptions{
		CandidateFilter: func(w Workflow) bool { return strings.EqualFold(w.Status, "ACTIVE") },
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not eligible")
	assert.Contains(t, err.Error(), "PAUSED")
}

func TestResolveWorkflowRow_ByName(t *testing.T) {
	t.Parallel()

	lookup := &fakeLookup{all: []Workflow{
		{UUID: "u1", Name: "my-workflow", WorkflowID: testWorkflowID1, Status: "ACTIVE", WorkflowSource: "private"},
		{UUID: "u2", Name: "my-workflow-staging", WorkflowID: testWorkflowID2, Status: "ACTIVE", WorkflowSource: "private"},
	}}

	// Exact match wins even when the search returns contains-matches.
	row, err := ResolveWorkflowRow(context.Background(), lookup, "my-workflow", RowResolveOptions{})
	require.NoError(t, err)
	assert.Equal(t, "u1", row.UUID)
	assert.Equal(t, "my-workflow", lookup.lastSearchName)

	_, err = ResolveWorkflowRow(context.Background(), lookup, "missing", RowResolveOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no workflow found with name "missing"`)
}

func TestResolveWorkflowRow_ByName_Ambiguous(t *testing.T) {
	t.Parallel()

	lookup := &fakeLookup{all: []Workflow{
		{UUID: "u1", Name: "dup", WorkflowID: testWorkflowID1, Status: "ACTIVE", WorkflowSource: "private"},
		{UUID: "u2", Name: "dup", WorkflowID: testWorkflowID2, Status: "ACTIVE", WorkflowSource: "contract:16015286601757825753:0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D"},
	}}

	// Non-interactive ambiguity → actionable error.
	_, err := ResolveWorkflowRow(context.Background(), lookup, "dup", RowResolveOptions{NonInteractive: true, Registries: testRegistries()})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple workflows named")
	assert.Contains(t, err.Error(), "--registry")

	// Interactive ambiguity → picker stub decides.
	picked := false
	row, err := ResolveWorkflowRow(context.Background(), lookup, "dup", RowResolveOptions{
		Registries: testRegistries(),
		Pick: func(candidates []Workflow, title string, registries []*tenantctx.Registry) (*Workflow, error) {
			picked = true
			require.Len(t, candidates, 2)
			return &candidates[1], nil
		},
	})
	require.NoError(t, err)
	assert.True(t, picked)
	assert.Equal(t, "u2", row.UUID)
}

func TestResolveWorkflowRow_ByName_RegistryFilter(t *testing.T) {
	t.Parallel()

	lookup := &fakeLookup{all: []Workflow{
		{UUID: "u1", Name: "dup", WorkflowID: testWorkflowID1, Status: "ACTIVE", WorkflowSource: "private"},
		{UUID: "u2", Name: "dup", WorkflowID: testWorkflowID2, Status: "ACTIVE", WorkflowSource: "contract:16015286601757825753:0x51D3acf4526e014deBf9884159A57f63Fc0Ca49D"},
	}}

	// Registry filter disambiguates without a picker.
	row, err := ResolveWorkflowRow(context.Background(), lookup, "dup", RowResolveOptions{
		RegistryFilter: "onchain-sepolia",
		Registries:     testRegistries(),
		NonInteractive: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "u2", row.UUID)

	// Unknown registry ID → error listing available registries.
	_, err = ResolveWorkflowRow(context.Background(), lookup, "dup", RowResolveOptions{
		RegistryFilter: "nope",
		Registries:     testRegistries(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `registry "nope" not found`)
}

func TestResolveWorkflowRow_ByName_OwnerFilterPassthrough(t *testing.T) {
	t.Parallel()

	lookup := &fakeLookup{all: []Workflow{
		{UUID: "u1", Name: "wf", WorkflowID: testWorkflowID1, Status: "ACTIVE", WorkflowSource: "private"},
	}}

	_, err := ResolveWorkflowRow(context.Background(), lookup, "wf", RowResolveOptions{OwnerFilter: "0xOwner"})
	require.NoError(t, err)
	assert.Equal(t, "0xOwner", lookup.lastSearchOwner)
}

func TestResolveWorkflowRow_ByName_OnlyDeleted(t *testing.T) {
	t.Parallel()

	lookup := &fakeLookup{all: []Workflow{
		{UUID: "u1", Name: "gone", WorkflowID: testWorkflowID1, Status: "DELETED", WorkflowSource: "private"},
	}}

	_, err := ResolveWorkflowRow(context.Background(), lookup, "gone", RowResolveOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "none of its deployments are eligible")
}

func TestResolveWorkflowRow_Picker(t *testing.T) {
	t.Parallel()

	lookup := &fakeLookup{all: []Workflow{
		{UUID: "u1", Name: "wf-one", WorkflowID: testWorkflowID1, Status: "ACTIVE", WorkflowSource: "private"},
		{UUID: "u2", Name: "wf-two", WorkflowID: testWorkflowID2, Status: "PAUSED", WorkflowSource: "private"},
		{UUID: "u3", Name: "wf-deleted", WorkflowID: "aaaa21b8b3e117e31f3a3e8a0795225cbde6c00283a84395117669691f2b7856", Status: "DELETED", WorkflowSource: "private"},
	}}

	// Non-interactive with no arg → error.
	_, err := ResolveWorkflowRow(context.Background(), lookup, "", RowResolveOptions{NonInteractive: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required in non-interactive mode")

	// Deleted rows are excluded from picker candidates.
	var gotCandidates []Workflow
	row, err := ResolveWorkflowRow(context.Background(), lookup, "", RowResolveOptions{
		Pick: func(candidates []Workflow, title string, registries []*tenantctx.Registry) (*Workflow, error) {
			gotCandidates = candidates
			return &candidates[0], nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "u1", row.UUID)
	require.Len(t, gotCandidates, 2)
	for _, c := range gotCandidates {
		assert.NotEqual(t, "DELETED", c.Status)
	}

	// Candidate filter applies to the picker list too.
	row, err = ResolveWorkflowRow(context.Background(), lookup, "", RowResolveOptions{
		CandidateFilter: func(w Workflow) bool { return strings.EqualFold(w.Status, "ACTIVE") },
		Pick: func(candidates []Workflow, title string, registries []*tenantctx.Registry) (*Workflow, error) {
			require.Len(t, candidates, 1)
			return &candidates[0], nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "u1", row.UUID)

	// Nothing eligible.
	_, err = ResolveWorkflowRow(context.Background(), lookup, "", RowResolveOptions{
		CandidateFilter: func(w Workflow) bool { return false },
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no eligible workflows found")
}

func TestResolveWorkflowRow_LookupErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")

	_, err := ResolveWorkflowRow(context.Background(), &fakeLookup{listErr: boom}, testWorkflowID1, RowResolveOptions{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, boom))

	_, err = ResolveWorkflowRow(context.Background(), &fakeLookup{listErr: boom}, "", RowResolveOptions{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, boom))

	_, err = ResolveWorkflowRow(context.Background(), &fakeLookup{searchErr: boom}, "wf", RowResolveOptions{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, boom))
}

func TestShortID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "abc", ShortID("abc"))
	assert.Equal(t, "00da21…7856", ShortID(testWorkflowID1))
	assert.Equal(t, fmt.Sprintf("%s…%s", testWorkflowID2[:6], testWorkflowID2[len(testWorkflowID2)-4:]), ShortID(testWorkflowID2))
}

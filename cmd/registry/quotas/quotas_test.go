package quotas_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/machinebox/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/cmd/registry/quotas"
	"github.com/smartcontractkit/cre-cli/internal/client/registryquotaclient"
	"github.com/smartcontractkit/cre-cli/internal/tenantctx"
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

const (
	registryAddress = "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5"
	chainSelector   = "5009297550715157269"
	ownerAddress    = "0x1111111111111111111111111111111111111111"
	pendingAddress  = "0x2222222222222222222222222222222222222222"
)

type fakeExecutor struct {
	data  string
	err   error
	calls int
}

func (f *fakeExecutor) Execute(_ context.Context, _ *graphql.Request, resp any) error {
	f.calls++
	if f.data != "" {
		if err := json.Unmarshal([]byte(f.data), resp); err != nil {
			return err
		}
	}
	return f.err
}

func strPtr(s string) *string { return &s }

func tenantContext() *tenantctx.EnvironmentContext {
	return &tenantctx.EnvironmentContext{Registries: []*tenantctx.Registry{
		{
			ID:            "onchain:ethereum-mainnet",
			Label:         "ethereum-mainnet (0x4Ac5...7e5)",
			Type:          "on-chain",
			ChainSelector: strPtr(chainSelector),
			// Lower case on purpose: matching must ignore address case.
			Address: strPtr(strings.ToLower(registryAddress)),
		},
		{ID: "private", Label: "Private (Chainlink-hosted)", Type: "off-chain"},
	}}
}

const fullData = `{"getRegistryQuotas": {
  "offChainRegistries": [{"workflows": [
    {"donFamily": "zone-b", "active": 4, "limit": 0},
    {"donFamily": "zone-a", "active": 4, "limit": 5}
  ]}],
  "onChainRegistries": [{"chainSelector": "5009297550715157269", "address": "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5",
    "owners": [{"owner": "0x1111111111111111111111111111111111111111", "workflows": [{"donFamily": "zone-a", "active": 4, "limit": 3}]}]}],
  "linkedOwners": {
    "linked": [{"address": "0x1111111111111111111111111111111111111111", "chainSelector": "5009297550715157269", "workflowRegistryAddress": "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5"}],
    "pending": [{"address": "0x2222222222222222222222222222222222222222", "chainSelector": "5009297550715157269", "workflowRegistryAddress": "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5"}],
    "limit": 3
  }
}}`

// run executes the handler and returns what it wrote to stdout and stderr.
func run(t *testing.T, exec *fakeExecutor, registryID, format string) (string, string, error) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout, os.Stderr = outW, errW

	err := quotas.NewHandler(exec, tenantContext()).Execute(context.Background(), registryID, format)

	outW.Close()
	errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	stdout, _ := io.ReadAll(outR)
	stderr, _ := io.ReadAll(errR)
	return string(stdout), string(stderr), err
}

func TestQuotas_Text(t *testing.T) {
	stdout, _, err := run(t, &fakeExecutor{data: fullData}, "", "")
	require.NoError(t, err)

	for _, want := range []string{
		"Private (Chainlink-hosted)",
		"zone-a  4 / 5 active, 1 remaining",
		"zone-b  disabled (4 active)",
		"every DON family shows the same total",
		"ethereum-mainnet (0x4Ac5...7e5)",
		ownerAddress + " (linked)",
		"4 / 3 active (over limit)",
		"Linked owners (all on-chain registries)",
		"2 / 3 used (1 linked, 1 pending), 1 remaining",
		pendingAddress + "  pending",
	} {
		assert.Contains(t, stdout, want)
	}
	assert.Less(t, strings.Index(stdout, "zone-a"), strings.Index(stdout, "zone-b"), "DON families are sorted")
	assert.NotContains(t, stdout, "user context")
}

func TestQuotas_JSON(t *testing.T) {
	stdout, _, err := run(t, &fakeExecutor{data: fullData}, "", workflowresolve.OutputFormatJSON)
	require.NoError(t, err)

	var report struct {
		Registries []struct {
			ID        string            `json:"id"`
			Type      string            `json:"type"`
			Workflows []json.RawMessage `json:"workflows"`
			Owners    []json.RawMessage `json:"owners"`
		} `json:"registries"`
		LinkedOwners *struct {
			Limit int `json:"limit"`
		} `json:"linkedOwners"`
		Warnings []string `json:"warnings"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &report), stdout)
	require.Len(t, report.Registries, 2)
	assert.Equal(t, "onchain:ethereum-mainnet", report.Registries[0].ID)
	assert.Len(t, report.Registries[0].Owners, 1)
	assert.Equal(t, "private", report.Registries[1].ID)
	assert.Len(t, report.Registries[1].Workflows, 2)
	require.NotNil(t, report.LinkedOwners)
	assert.Equal(t, 3, report.LinkedOwners.Limit)
	assert.Empty(t, report.Warnings)
}

func TestQuotas_FilterByRegistry(t *testing.T) {
	stdout, _, err := run(t, &fakeExecutor{data: fullData}, "private", workflowresolve.OutputFormatJSON)
	require.NoError(t, err)

	var report map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &report))
	var registries []map[string]any
	require.NoError(t, json.Unmarshal(report["registries"], &registries))
	require.Len(t, registries, 1)
	assert.Equal(t, "private", registries[0]["id"])
	_, hasLinked := report["linkedOwners"]
	assert.False(t, hasLinked, "linked owners only apply to on-chain registries")
}

func TestQuotas_UnknownRegistry(t *testing.T) {
	exec := &fakeExecutor{data: fullData}
	_, _, err := run(t, exec, "nope", "")
	require.ErrorContains(t, err, `unknown registry "nope"; valid IDs: onchain:ethereum-mainnet, private`)
	assert.Equal(t, 0, exec.calls, "the API is not called for an unknown registry")
}

func TestQuotas_PartialResult(t *testing.T) {
	data := `{"getRegistryQuotas": {
	  "offChainRegistries": [{"workflows": [{"donFamily": "zone-a", "active": 1, "limit": 3}]}],
	  "onChainRegistries": [{"chainSelector": "5009297550715157269", "address": "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5", "owners": null}],
	  "linkedOwners": null
	}}`
	exec := &fakeExecutor{data: data, err: errors.New("cre api error: on-chain registry temporarily unavailable")}

	t.Run("text", func(t *testing.T) {
		stdout, stderr, err := run(t, exec, "", "")
		require.NoError(t, err, "a partial result is not a failure")
		assert.Contains(t, stdout, "zone-a  1 / 3 active, 2 remaining")
		assert.Contains(t, stderr, "On-chain workflow quota unavailable")
		assert.Contains(t, stderr, "Linked-owner quota unavailable")
		assert.Contains(t, stderr, "on-chain registry temporarily unavailable")
	})

	t.Run("json keeps unavailable parts as null", func(t *testing.T) {
		stdout, _, err := run(t, exec, "", workflowresolve.OutputFormatJSON)
		require.NoError(t, err)

		var report map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(stdout), &report))
		assert.JSONEq(t, `null`, string(report["linkedOwners"]))
		assert.Contains(t, string(report["registries"]), `"owners": null`)
		assert.Contains(t, string(report["warnings"]), "on-chain registry temporarily unavailable")
	})
}

func TestQuotas_RegistryMissingFromUserContext(t *testing.T) {
	data := `{"getRegistryQuotas": {
	  "offChainRegistries": [],
	  "onChainRegistries": [
	    {"chainSelector": "5009297550715157269", "address": "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5", "owners": []},
	    {"chainSelector": "16015286601757825753", "address": "0x3333333333333333333333333333333333333333", "owners": []}
	  ],
	  "linkedOwners": {"linked": [], "pending": [], "limit": 1}
	}}`
	stdout, _, err := run(t, &fakeExecutor{data: data}, "", "")
	require.NoError(t, err)

	assert.Contains(t, stdout, "chain 16015286601757825753 (0x3333...3333)")
	assert.Contains(t, stdout, "Run `cre login` to refresh it")
	assert.Contains(t, stdout, "No linked owners")
	assert.NotContains(t, stdout, "Private (Chainlink-hosted)", "a registry without a quota is not shown")
}

func TestQuotas_FetchErrors(t *testing.T) {
	t.Run("permission denied", func(t *testing.T) {
		_, _, err := run(t, &fakeExecutor{err: errors.New("cre api error: unauthorized: user does not have platform:limits:read permission")}, "", "")
		require.ErrorIs(t, err, registryquotaclient.ErrPermissionDenied)
	})
}

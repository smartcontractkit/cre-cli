package registryquotaclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/machinebox/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/cre-cli/internal/client/registryquotaclient"
)

// fakeExecutor decodes data into the response like the GraphQL client does, which
// also decodes data when the response carries errors.
type fakeExecutor struct {
	data string
	err  error
}

func (f *fakeExecutor) Execute(_ context.Context, _ *graphql.Request, resp any) error {
	if f.data != "" {
		if err := json.Unmarshal([]byte(f.data), resp); err != nil {
			return err
		}
	}
	return f.err
}

const fullData = `{"getRegistryQuotas": {
  "offChainRegistries": [{"workflows": [{"donFamily": "zone-a", "active": 1, "limit": 3}]}],
  "onChainRegistries": [{"chainSelector": "5009297550715157269", "address": "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5",
    "owners": [{"owner": "0x1111111111111111111111111111111111111111", "workflows": [{"donFamily": "zone-a", "active": 2, "limit": 3}]}]}],
  "linkedOwners": {"linked": [{"address": "0x1111111111111111111111111111111111111111", "chainSelector": 5009297550715157269,
    "workflowRegistryAddress": "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5"}], "pending": [], "limit": 1}
}}`

func TestFetch_Success(t *testing.T) {
	q, err := registryquotaclient.Fetch(context.Background(), &fakeExecutor{data: fullData})
	require.NoError(t, err)

	require.Len(t, q.OffChainRegistries, 1)
	assert.Equal(t, []registryquotaclient.DonFamilyQuota{{DonFamily: "zone-a", Active: 1, Limit: 3}}, q.OffChainRegistries[0].Workflows)
	require.Len(t, q.OnChainRegistries, 1)
	assert.Equal(t, registryquotaclient.ChainSelector("5009297550715157269"), q.OnChainRegistries[0].ChainSelector)
	require.Len(t, q.OnChainRegistries[0].Owners, 1)
	require.NotNil(t, q.LinkedOwners)
	assert.Equal(t, 1, q.LinkedOwners.Limit)
	assert.Equal(t, registryquotaclient.ChainSelector("5009297550715157269"), q.LinkedOwners.Linked[0].ChainSelector, "numeric selectors keep full precision")
	assert.NotNil(t, q.LinkedOwners.Pending)
}

func TestFetch_PartialData(t *testing.T) {
	data := `{"getRegistryQuotas": {
	  "offChainRegistries": [{"workflows": [{"donFamily": "zone-a", "active": 1, "limit": 3}]}],
	  "onChainRegistries": [{"chainSelector": "1", "address": "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5", "owners": null}],
	  "linkedOwners": null
	}}`
	q, err := registryquotaclient.Fetch(context.Background(), &fakeExecutor{data: data, err: errors.New("cre api error: on-chain registry temporarily unavailable")})

	var partial *registryquotaclient.PartialError
	require.ErrorAs(t, err, &partial)
	require.NotNil(t, q)
	assert.NotNil(t, q.OffChainRegistries[0].Workflows)
	assert.Nil(t, q.OnChainRegistries[0].Owners, "null owners stay nil so callers can show them as unavailable")
	assert.Nil(t, q.LinkedOwners)
}

func TestFetch_Errors(t *testing.T) {
	t.Run("missing permission", func(t *testing.T) {
		_, err := registryquotaclient.Fetch(context.Background(), &fakeExecutor{
			data: `{"getRegistryQuotas": null}`,
			err:  errors.New("cre api error: unauthorized: user does not have platform:limits:read permission"),
		})
		require.ErrorIs(t, err, registryquotaclient.ErrPermissionDenied)
	})

	t.Run("other error", func(t *testing.T) {
		_, err := registryquotaclient.Fetch(context.Background(), &fakeExecutor{err: errors.New("connection refused")})
		require.ErrorContains(t, err, "fetch registry quotas: connection refused")
		assert.NotErrorIs(t, err, registryquotaclient.ErrPermissionDenied)
	})

	t.Run("no data and no error", func(t *testing.T) {
		_, err := registryquotaclient.Fetch(context.Background(), &fakeExecutor{data: `{"getRegistryQuotas": null}`})
		require.ErrorContains(t, err, "no registry quotas")
	})
}

func TestChainSelector_RejectsInvalid(t *testing.T) {
	var c registryquotaclient.ChainSelector
	require.Error(t, json.Unmarshal([]byte(`-1`), &c))
	require.Error(t, json.Unmarshal([]byte(`true`), &c))
	require.NoError(t, json.Unmarshal([]byte(`null`), &c))
	assert.Equal(t, registryquotaclient.ChainSelector(""), c)
}

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
  "registryLimits": [
    {"quotaKey": "PerOrg.PrivateRegistry.WorkflowLimit", "service": "cre",
      "dimensions": [{"key": "registry", "value": "private"}, {"key": "donFamily", "value": "zone-a"}],
      "currentValue": "1", "totalLimit": "3"},
    {"quotaKey": "PerOwner.OnchainRegistry.WorkflowLimit", "service": "cre",
      "dimensions": [
        {"key": "registry", "value": "ethereum-mainnet"},
        {"key": "registryAddress", "value": "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5"},
        {"key": "owner", "value": "0x1111111111111111111111111111111111111111"},
        {"key": "donFamily", "value": "zone-a"}],
      "currentValue": "2", "totalLimit": "3"}
  ],
  "ownerLimits": [
    {"quotaKey": "PerOrg.OnchainRegistry.LinkedKeyLimit", "service": "cre",
      "dimensions": [], "currentValue": "1", "totalLimit": "1"}
  ]
}}`

func TestFetch_Success(t *testing.T) {
	q, err := registryquotaclient.Fetch(context.Background(), &fakeExecutor{data: fullData})
	require.NoError(t, err)

	require.Len(t, q.RegistryLimits, 2)
	private := q.RegistryLimits[0]
	assert.Equal(t, registryquotaclient.QuotaKeyPrivateWorkflowLimit, private.QuotaKey)
	assert.Equal(t, "cre", private.Service)
	assert.Equal(t, "private", private.Dimension(registryquotaclient.DimensionRegistry))
	assert.Equal(t, "zone-a", private.Dimension(registryquotaclient.DimensionDonFamily))
	require.NotNil(t, private.CurrentValue)
	assert.Equal(t, "1", private.CurrentValue.String())
	assert.Equal(t, "3", private.TotalLimit.String())

	onchain := q.RegistryLimits[1]
	assert.Equal(t, "ethereum-mainnet", onchain.Dimension(registryquotaclient.DimensionRegistry))
	assert.Equal(t, "0x4Ac54353FA4Fa961AfcC5ec4B118596d3305E7e5", onchain.Dimension(registryquotaclient.DimensionRegistryAddress))
	assert.Equal(t, "0x1111111111111111111111111111111111111111", onchain.Dimension(registryquotaclient.DimensionOwner))

	require.Len(t, q.OwnerLimits, 1)
	require.NotNil(t, q.OwnerLimits[0].CurrentValue)
	assert.Equal(t, "1", q.OwnerLimits[0].CurrentValue.String())
	assert.Equal(t, "1", q.OwnerLimits[0].TotalLimit.String())
}

func TestFetch_NullUsage(t *testing.T) {
	data := `{"getRegistryQuotas": {
	  "registryLimits": [{"quotaKey": "PerOrg.PrivateRegistry.WorkflowLimit",
	    "dimensions": [{"key": "donFamily", "value": "zone-a"}], "currentValue": null, "totalLimit": "3"}],
	  "ownerLimits": []
	}}`
	q, err := registryquotaclient.Fetch(context.Background(), &fakeExecutor{data: data})
	require.NoError(t, err)
	assert.Nil(t, q.RegistryLimits[0].CurrentValue, "null usage stays nil")
	assert.Equal(t, "3", q.RegistryLimits[0].TotalLimit.String())
}

func TestFetch_PartialData(t *testing.T) {
	data := `{"getRegistryQuotas": {
	  "registryLimits": [{"quotaKey": "PerOrg.PrivateRegistry.WorkflowLimit",
	    "dimensions": [{"key": "registry", "value": "private"}, {"key": "donFamily", "value": "zone-a"}],
	    "currentValue": "1", "totalLimit": "3"}],
	  "ownerLimits": []
	}}`
	err := errors.New("cre api error: quota PerOwner.OnchainRegistry.WorkflowLimit is not available")
	q, fetchErr := registryquotaclient.Fetch(context.Background(), &fakeExecutor{data: data, err: err})

	var partial *registryquotaclient.PartialError
	require.ErrorAs(t, fetchErr, &partial)
	require.NotNil(t, q)
	assert.Len(t, q.RegistryLimits, 1)
	assert.Empty(t, q.OwnerLimits)
}

func TestFailedQuotaKeys(t *testing.T) {
	err := errors.New("cre api error: quota PerOrg.PrivateRegistry.WorkflowLimit is not available; " +
		"quota PerOrg.OnchainRegistry.LinkedKeyLimit requires the platform:owner_address:read permission")
	assert.Equal(t, []string{
		registryquotaclient.QuotaKeyPrivateWorkflowLimit,
		registryquotaclient.QuotaKeyLinkedOwnersLimit,
	}, registryquotaclient.FailedQuotaKeys(err))
	assert.Empty(t, registryquotaclient.FailedQuotaKeys(errors.New("connection refused")))
}

func TestFetch_Errors(t *testing.T) {
	t.Run("missing permission", func(t *testing.T) {
		_, err := registryquotaclient.Fetch(context.Background(), &fakeExecutor{
			data: `{"getRegistryQuotas": null}`,
			err:  errors.New("cre api error: unauthorized: user does not have sufficient permissions. Requires one of: [platform:limits:read/cre platform:owner_address:read])"),
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

func TestAmount_JSON(t *testing.T) {
	var value struct {
		Current *registryquotaclient.Amount `json:"current"`
		Limit   registryquotaclient.Amount  `json:"limit"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"current": "12.50", "limit": "25"}`), &value))
	require.NotNil(t, value.Current)
	assert.Equal(t, "12.5", value.Current.String(), "trailing zeros are dropped")
	assert.Equal(t, "25", value.Limit.String())

	raw, err := json.Marshal(value)
	require.NoError(t, err)
	assert.JSONEq(t, `{"current": 12.5, "limit": 25}`, string(raw), "amounts are JSON numbers")

	var bad registryquotaclient.Amount
	require.ErrorContains(t, json.Unmarshal([]byte(`"not a number"`), &bad), "invalid quota amount")
	require.ErrorContains(t, json.Unmarshal([]byte(`true`), &bad), "invalid quota amount")
}

func TestAmount_Arithmetic(t *testing.T) {
	var a, b registryquotaclient.Amount
	require.NoError(t, a.UnmarshalJSON([]byte(`"5"`)))
	require.NoError(t, b.UnmarshalJSON([]byte(`"1.5"`)))

	assert.Equal(t, 1, a.Cmp(b))
	assert.Equal(t, "3.5", a.Sub(b).String())
	assert.False(t, b.IsZero())
}

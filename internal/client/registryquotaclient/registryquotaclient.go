// Package registryquotaclient fetches registry quotas (workflow and linked-owner usage
// against the effective limits) from the CRE API.
package registryquotaclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/machinebox/graphql"
	"github.com/shopspring/decimal"
)

const query = `query GetRegistryQuotas {
  getRegistryQuotas {
    registryLimits {
      quotaKey
      service
      dimensions { key value }
      currentValue
      totalLimit
    }
    ownerLimits {
      quotaKey
      service
      dimensions { key value }
      currentValue
      totalLimit
    }
  }
}`

// Quota keys, see https://docs.chain.link/cre/service-quotas.
const (
	QuotaKeyPrivateWorkflowLimit = "PerOrg.PrivateRegistry.WorkflowLimit"
	QuotaKeyOnchainWorkflowLimit = "PerOwner.OnchainRegistry.WorkflowLimit"
	QuotaKeyLinkedOwnersLimit    = "PerOrg.OnchainRegistry.LinkedKeyLimit"
)

// Quota dimension keys. Each limit identifies where it applies by these.
const (
	DimensionRegistry        = "registry"
	DimensionRegistryAddress = "registryAddress"
	DimensionOwner           = "owner"
	DimensionDonFamily       = "donFamily"
)

// ErrPermissionDenied is returned when the caller lacks both the CRE limits and the
// owner address permissions.
var ErrPermissionDenied = errors.New("you do not have permission to read registry quotas (requires platform:limits:read/cre or platform:owner_address:read)")

// Executor runs a GraphQL request. It is satisfied by graphqlclient.Client.
type Executor interface {
	Execute(ctx context.Context, req *graphql.Request, resp any) error
}

// Quotas is the getRegistryQuotas result. A quota that could not be read is left out of
// its list; a *PartialError names at least one such quota.
type Quotas struct {
	RegistryLimits []ServiceUsageLimit `json:"registryLimits"`
	OwnerLimits    []ServiceUsageLimit `json:"ownerLimits"`
}

// ServiceUsageLimit is one registry quota: usage and limit for a set of dimensions.
type ServiceUsageLimit struct {
	QuotaKey     string           `json:"quotaKey"`
	Service      string           `json:"service"`
	Dimensions   []QuotaDimension `json:"dimensions"`
	CurrentValue *Amount          `json:"currentValue"`
	TotalLimit   Amount           `json:"totalLimit"`
}

// Dimension returns the value of a dimension, or "" when it is not set.
func (l ServiceUsageLimit) Dimension(key string) string {
	for _, d := range l.Dimensions {
		if d.Key == key {
			return d.Value
		}
	}
	return ""
}

// QuotaDimension places a limit, e.g. registry or donFamily.
type QuotaDimension struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Amount is a non-negative decimal amount of quota units, sent by the API as a string
// (e.g. "25" or "0.5"). It marshals as a JSON number.
type Amount struct {
	decimal.Decimal
}

// UnmarshalJSON reads the API's string form of a decimal amount.
func (a *Amount) UnmarshalJSON(raw []byte) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return fmt.Errorf("invalid quota amount %s: %w", raw, err)
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return fmt.Errorf("invalid quota amount %s: %w", raw, err)
	}
	a.Decimal = d
	return nil
}

// MarshalJSON writes the amount as a JSON number.
func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(a.Decimal.String()), nil
}

// Sub returns a-b.
func (a Amount) Sub(b Amount) Amount {
	return Amount{a.Decimal.Sub(b.Decimal)}
}

// Cmp compares a and b, returning -1, 0 or +1.
func (a Amount) Cmp(b Amount) int {
	return a.Decimal.Cmp(b.Decimal)
}

// IsZero reports whether the amount is zero.
func (a Amount) IsZero() bool {
	return a.Decimal.IsZero()
}

// PartialError is returned together with the quotas when the API returned data but
// could not read at least one quota. Unreadable quotas are missing from the lists.
type PartialError struct {
	Err error
}

func (e *PartialError) Error() string { return e.Err.Error() }
func (e *PartialError) Unwrap() error { return e.Err }

// FailedQuotaKeys returns the quotas named in a partial error. The GraphQL client keeps
// only the error message, and every quota error names its quota key.
func FailedQuotaKeys(err error) []string {
	if err == nil {
		return nil
	}
	msg := err.Error()
	keys := []string{}
	for _, key := range []string{
		QuotaKeyPrivateWorkflowLimit,
		QuotaKeyOnchainWorkflowLimit,
		QuotaKeyLinkedOwnersLimit,
	} {
		if strings.Contains(msg, key) {
			keys = append(keys, key)
		}
	}
	return keys
}

// Fetch reads the registry quotas of the caller's organization.
//
// On success it returns the quotas and a nil error. When the API returned data with
// errors, it returns the quotas and a *PartialError. Otherwise it returns nil and an
// error, which is ErrPermissionDenied when the caller lacks the required permission.
func Fetch(ctx context.Context, client Executor) (*Quotas, error) {
	var resp struct {
		GetRegistryQuotas *Quotas `json:"getRegistryQuotas"`
	}
	err := client.Execute(ctx, graphql.NewRequest(query), &resp)
	if err == nil {
		if resp.GetRegistryQuotas == nil {
			return nil, errors.New("CRE API returned no registry quotas")
		}
		return resp.GetRegistryQuotas, nil
	}

	// The GraphQL client still decodes data when the response also carries errors.
	if resp.GetRegistryQuotas != nil {
		return resp.GetRegistryQuotas, &PartialError{Err: err}
	}
	return nil, classify(err)
}

func classify(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "platform:limits:read") || strings.Contains(msg, "platform:owner_address:read") {
		return fmt.Errorf("%w: %w", ErrPermissionDenied, err)
	}
	return fmt.Errorf("fetch registry quotas: %w", err)
}

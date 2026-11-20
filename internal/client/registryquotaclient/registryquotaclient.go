// Package registryquotaclient fetches registry quotas (workflow and linked-owner usage
// against the effective limits) from the CRE API.
package registryquotaclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/machinebox/graphql"
)

const query = `query GetRegistryQuotas {
  getRegistryQuotas {
    offChainRegistries {
      workflows { donFamily active limit }
    }
    onChainRegistries {
      chainSelector
      address
      owners {
        owner
        workflows { donFamily active limit }
      }
    }
    linkedOwners {
      linked { address chainSelector workflowRegistryAddress }
      pending { address chainSelector workflowRegistryAddress }
      limit
    }
  }
}`

// ErrNotSupported is returned when the CRE API does not serve registry quotas yet.
var ErrNotSupported = errors.New("registry quotas are not available from the CRE API yet")

// ErrPermissionDenied is returned when the caller lacks the platform:limits:read permission.
var ErrPermissionDenied = errors.New("you do not have permission to read registry quotas (requires platform:limits:read)")

// Executor runs a GraphQL request. It is satisfied by graphqlclient.Client.
type Executor interface {
	Execute(ctx context.Context, req *graphql.Request, resp any) error
}

// Quotas is the getRegistryQuotas result. A nil slice or pointer on a field the
// server resolves separately (OffChainRegistry.Workflows, OnChainRegistry.Owners,
// LinkedOwners) means the server could not read it; an empty slice means there is
// nothing to show. LinkedOwners is also nil when there is no on-chain registry.
type Quotas struct {
	OffChainRegistries []OffChainRegistry `json:"offChainRegistries"`
	OnChainRegistries  []OnChainRegistry  `json:"onChainRegistries"`
	LinkedOwners       *LinkedOwners      `json:"linkedOwners"`
}

type OffChainRegistry struct {
	Workflows []DonFamilyQuota `json:"workflows"`
}

type OnChainRegistry struct {
	ChainSelector ChainSelector `json:"chainSelector"`
	Address       string        `json:"address"`
	Owners        []OwnerQuota  `json:"owners"`
}

type OwnerQuota struct {
	Owner     string           `json:"owner"`
	Workflows []DonFamilyQuota `json:"workflows"`
}

type DonFamilyQuota struct {
	DonFamily string `json:"donFamily"`
	Active    int    `json:"active"`
	Limit     int    `json:"limit"`
}

type LinkedOwners struct {
	Linked  []LinkedOwner `json:"linked"`
	Pending []LinkedOwner `json:"pending"`
	Limit   int           `json:"limit"`
}

type LinkedOwner struct {
	Address                 string        `json:"address"`
	ChainSelector           ChainSelector `json:"chainSelector"`
	WorkflowRegistryAddress string        `json:"workflowRegistryAddress"`
}

// ChainSelector is a chain selector as a decimal string. The API may send it as a
// JSON string or number; it is kept as a string to avoid precision loss.
type ChainSelector string

func (c *ChainSelector) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*c = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		*c = ChainSelector(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return fmt.Errorf("invalid chain selector %s: %w", raw, err)
	}
	if _, err := strconv.ParseUint(n.String(), 10, 64); err != nil {
		return fmt.Errorf("invalid chain selector %s: %w", raw, err)
	}
	*c = ChainSelector(n.String())
	return nil
}

// PartialError is returned together with the quotas when the API returned data but
// could not read every part of it. Unreadable parts are nil in the returned Quotas.
type PartialError struct {
	Err error
}

func (e *PartialError) Error() string { return e.Err.Error() }
func (e *PartialError) Unwrap() error { return e.Err }

// Fetch reads the registry quotas of the caller's organization.
//
// On success it returns the quotas and a nil error. When the API returned data with
// errors, it returns the quotas and a *PartialError. Otherwise it returns nil and an
// error, which is ErrNotSupported or ErrPermissionDenied where those apply.
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
	switch {
	case strings.Contains(msg, "Cannot query field") && strings.Contains(msg, "getRegistryQuotas"):
		return fmt.Errorf("%w: %w", ErrNotSupported, err)
	case strings.Contains(msg, "platform:limits:read"):
		return fmt.Errorf("%w: %w", ErrPermissionDenied, err)
	default:
		return fmt.Errorf("fetch registry quotas: %w", err)
	}
}

package authvalidation

import (
	"context"
	"fmt"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
)

const queryCreOrganizationInfo = `
query GetCreOrganizationInfo {
	getCreOrganizationInfo {
		orgId
		derivedWorkflowOwners
	}
}`

// ValidationResult holds the data returned by credential validation.
type ValidationResult struct {
	OrgID                string
	DerivedWorkflowOwner string
}

// Validator validates authentication credentials
type Validator struct {
	gqlClient *graphqlclient.Client
	log       *zerolog.Logger
}

// NewValidator creates a new credential validator
func NewValidator(creds *credentials.Credentials, environmentSet *environments.EnvironmentSet, log *zerolog.Logger) *Validator {
	gqlClient := graphqlclient.New(creds, environmentSet, log)
	return &Validator{
		gqlClient: gqlClient,
		log:       log,
	}
}

// ValidateCredentials validates the provided credentials by making a lightweight GraphQL query
// and returns organization info including the derived workflow owner.
// The GraphQL client automatically handles token refresh if needed.
func (v *Validator) ValidateCredentials(validationCtx context.Context, creds *credentials.Credentials) (*ValidationResult, error) {
	if creds == nil {
		return nil, fmt.Errorf("credentials not provided")
	}

	if creds.IsValidated {
		return nil, nil
	}

	req := graphql.NewRequest(queryCreOrganizationInfo)

	var respEnvelope struct {
		GetCreOrganizationInfo struct {
			OrgID                 string   `json:"orgId"`
			DerivedWorkflowOwners []string `json:"derivedWorkflowOwners"`
		} `json:"getCreOrganizationInfo"`
	}

	if err := v.gqlClient.Execute(validationCtx, req, &respEnvelope); err != nil {
		return nil, fmt.Errorf("authentication failed: unable to retrieve organization info. Your account may not be fully set up yet — please try again in a few minutes: %w", err)
	}

	info := respEnvelope.GetCreOrganizationInfo

	if info.OrgID == "" || len(info.DerivedWorkflowOwners) == 0 {
		return nil, fmt.Errorf("authentication failed: unable to retrieve organization info. Your account may not be fully set up yet — please try again in a few minutes")
	}

	result := &ValidationResult{
		OrgID:                info.OrgID,
		DerivedWorkflowOwner: info.DerivedWorkflowOwners[0],
	}

	creds.OrgID = result.OrgID

	return result, nil
}

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

const queryOrganization = `
query GetOrganizationDetails  {
	getOrganization {
		organizationId
	}
}`

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
// The GraphQL client automatically handles token refresh if needed
func (v *Validator) ValidateCredentials(validationCtx context.Context, creds *credentials.Credentials) error {
	if creds == nil {
		return fmt.Errorf("credentials not provided")
	}

	// Skip validation if already validated
	if creds.IsValidated {
		return nil
	}

	req := graphql.NewRequest(queryOrganization)

	var respEnvelope struct {
		GetOrganization struct {
			OrganizationID string `json:"organizationId"`
		} `json:"getOrganization"`
	}

	if err := v.gqlClient.Execute(validationCtx, req, &respEnvelope); err != nil {
		return fmt.Errorf("authentication validation failed: %w", err)
	}

	return nil
}

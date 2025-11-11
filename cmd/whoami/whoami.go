package whoami

import (
	"context"
	"fmt"

	"github.com/machinebox/graphql"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/smartcontractkit/cre-cli/internal/client/graphqlclient"
	"github.com/smartcontractkit/cre-cli/internal/credentials"
	"github.com/smartcontractkit/cre-cli/internal/environments"
	"github.com/smartcontractkit/cre-cli/internal/runtime"
)

const queryGetAccountDetails = `
query GetAccountDetails {
	getAccountDetails {
		userId
		organizationId
		emailAddress
		displayName
		memberType
		memberStatus
		createdAt
		updatedAt
		invitedByUser
		invitedAt
		joinedAt
		removedByUser
		removedAt
	}
}`

const getOrganization = `
query GetOrganization {
	getOrganization {
		organizationId
		displayName
	}
}`

func New(runtimeCtx *runtime.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show your current account details",
		Long:  "Fetches your account details (email and organization ID).",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			h := NewHandler(runtimeCtx)
			return h.Execute(cmd.Context())
		},
	}
	return cmd
}

type Handler struct {
	log            *zerolog.Logger
	credentials    *credentials.Credentials
	environmentSet *environments.EnvironmentSet
}

func NewHandler(ctx *runtime.Context) *Handler {
	return &Handler{
		log:            ctx.Logger,
		credentials:    ctx.Credentials,
		environmentSet: ctx.EnvironmentSet,
	}
}

func (h *Handler) Execute(ctx context.Context) error {
	client := graphqlclient.New(h.credentials, h.environmentSet, h.log)
	reqGetAccountDetails := graphql.NewRequest(queryGetAccountDetails)
	reqGetOrganization := graphql.NewRequest(getOrganization)

	var respEnvelopeGetAccountDetails struct {
		GetAccountDetails struct {
			Username       string `json:"username"`
			OrganizationID string `json:"organizationID"`
			EmailAddress   string `json:"emailAddress"`
		} `json:"getAccountDetails"`
	}

	var respEnvelopeGetOrganization struct {
		GetOrganization struct {
			OrganizationID string `json:"organizationID"`
			DisplayName    string `json:"displayName"`
		} `json:"getOrganization"`
	}

	if err := client.Execute(ctx, reqGetAccountDetails, &respEnvelopeGetAccountDetails); err != nil {
		return fmt.Errorf("graphql request failed: %w", err)
	}

	if err := client.Execute(ctx, reqGetOrganization, &respEnvelopeGetOrganization); err != nil {
		return fmt.Errorf("graphql request failed: %w", err)
	}

	fmt.Println("")
	fmt.Println("Account details retrieved:")
	fmt.Println("")
	fmt.Printf("\tEmail:             %s\n", respEnvelopeGetAccountDetails.GetAccountDetails.EmailAddress)
	fmt.Printf("\tOrganization ID:   %s\n", respEnvelopeGetAccountDetails.GetAccountDetails.OrganizationID)
	fmt.Printf("\tOrganization Name: %s\n", respEnvelopeGetOrganization.GetOrganization.DisplayName)
	fmt.Println("")

	return nil
}

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
	client := graphqlclient.New(h.credentials, h.environmentSet)
	req := graphql.NewRequest(queryGetAccountDetails)

	var respEnvelope struct {
		GetAccountDetails struct {
			Username       string `json:"username"`
			OrganizationID string `json:"organizationID"`
			EmailAddress   string `json:"emailAddress"`
		} `json:"getAccountDetails"`
	}

	if err := client.Execute(ctx, req, &respEnvelope); err != nil {
		return fmt.Errorf("graphql request failed: %w", err)
	}

	h.log.Info().Msg("")
	h.log.Info().Msg("	Account details retrieved:")
	h.log.Info().Msg("")
	h.log.Info().Msgf("   	Email:           %s", respEnvelope.GetAccountDetails.EmailAddress)
	h.log.Info().Msgf("   	Organization ID: %s", respEnvelope.GetAccountDetails.OrganizationID)
	h.log.Info().Msg("")

	return nil
}

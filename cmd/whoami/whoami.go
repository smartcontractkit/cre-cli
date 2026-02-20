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
	"github.com/smartcontractkit/cre-cli/internal/ui"
)

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
	var query string
	if h.credentials.APIKey == "" {
		query = `
        query GetWhoamiDetails {
            getAccountDetails {
                emailAddress
            }
            getOrganization {
                displayName
				organizationId
            }
        }`
	} else {
		query = `
        query GetWhoamiDetails {
            getOrganization {
                displayName
				organizationId
            }
        }`
	}

	client := graphqlclient.New(h.credentials, h.environmentSet, h.log)
	req := graphql.NewRequest(query)

	var respEnvelope struct {
		GetAccountDetails *struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"getAccountDetails"`
		GetOrganization struct {
			DisplayName    string `json:"displayName"`
			OrganizationID string `json:"organizationId"`
		} `json:"getOrganization"`
	}

	spinner := ui.GlobalSpinner()
	spinner.Start("Fetching account details...")
	err := client.Execute(ctx, req, &respEnvelope)
	spinner.Stop()

	if err != nil {
		return fmt.Errorf("graphql request failed: %w", err)
	}

	ui.Line()
	ui.Title("Account Details")

	details := fmt.Sprintf("Organization ID:   %s\nOrganization Name: %s",
		respEnvelope.GetOrganization.OrganizationID,
		respEnvelope.GetOrganization.DisplayName)

	if respEnvelope.GetAccountDetails != nil {
		details = fmt.Sprintf("Email:             %s\n%s",
			respEnvelope.GetAccountDetails.EmailAddress,
			details)
	}

	ui.Box(details)
	ui.Line()

	return nil
}

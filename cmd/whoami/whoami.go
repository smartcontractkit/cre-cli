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

	if err := client.Execute(ctx, req, &respEnvelope); err != nil {
		return fmt.Errorf("graphql request failed: %w", err)
	}

	// Get deployment access status
	deployAccess, err := h.credentials.GetDeploymentAccessStatus()
	if err != nil {
		h.log.Debug().Err(err).Msg("failed to get deployment access status")
	}

	fmt.Println("")
	fmt.Println("Account details retrieved:")
	fmt.Println("")
	if respEnvelope.GetAccountDetails != nil {
		fmt.Printf("\tEmail:             %s\n", respEnvelope.GetAccountDetails.EmailAddress)
	}
	fmt.Printf("\tOrganization ID:   %s\n", respEnvelope.GetOrganization.OrganizationID)
	fmt.Printf("\tOrganization Name: %s\n", respEnvelope.GetOrganization.DisplayName)

	// Display deployment access status
	if deployAccess != nil {
		if deployAccess.HasAccess {
			fmt.Printf("\tDeploy Access:     Enabled\n")
		} else {
			fmt.Printf("\tDeploy Access:     Not enabled (run 'cre account access' to request)\n")
		}
	}

	fmt.Println("")

	return nil
}

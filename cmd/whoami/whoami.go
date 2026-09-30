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
	"github.com/smartcontractkit/cre-cli/internal/workflowresolve"
)

// Inputs holds resolved and validated flag values for whoami.
type Inputs struct {
	OutputFormat string
}

func resolveInputs(outputFormat string, jsonFlag bool) (Inputs, error) {
	outputFormat, err := workflowresolve.ResolveOutputFormat(outputFormat, jsonFlag)
	if err != nil {
		return Inputs{}, err
	}
	return Inputs{OutputFormat: outputFormat}, nil
}

// whoamiJSON is the stable JSON contract for `cre whoami --json`.
type whoamiJSON struct {
	OrganizationID   string `json:"organizationId"`
	OrganizationName string `json:"organizationName"`
	// Email is empty for API-key auth, which has no account details to fetch.
	Email       string `json:"email,omitempty"`
	Environment string `json:"environment"`
}

func New(runtimeCtx *runtime.Context) *cobra.Command {
	var outputFormat string
	var jsonFlag bool

	cmd := &cobra.Command{
		Use:     "whoami",
		Short:   "Show your current account details",
		Long:    "Fetches your account details (email and organization ID).",
		Example: "cre whoami\n  cre whoami --json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := resolveInputs(outputFormat, jsonFlag)
			if err != nil {
				return err
			}
			h := NewHandler(runtimeCtx)
			return h.Execute(cmd.Context(), inputs)
		},
	}

	cmd.Flags().StringVar(&outputFormat, "output", "", `Output format: "json" prints a JSON object to stdout`)
	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON (shorthand for --output=json)")

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

func (h *Handler) Execute(ctx context.Context, inputs Inputs) error {
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

	if inputs.OutputFormat == workflowresolve.OutputFormatJSON {
		out := whoamiJSON{
			OrganizationID:   respEnvelope.GetOrganization.OrganizationID,
			OrganizationName: respEnvelope.GetOrganization.DisplayName,
			Environment:      h.environmentSet.EnvLabel(),
		}
		if respEnvelope.GetAccountDetails != nil {
			out.Email = respEnvelope.GetAccountDetails.EmailAddress
		}
		return workflowresolve.PrintJSON(out)
	}

	// Get deployment access status
	deployAccess, err := h.credentials.GetDeploymentAccessStatus()
	if err != nil {
		h.log.Debug().Err(err).Msg("failed to get deployment access status")
	}

	ui.Line()
	ui.Title("Account Details")
	ui.EnvContext(h.environmentSet.EnvLabel())

	details := fmt.Sprintf("Organization ID:   %s\nOrganization Name: %s",
		respEnvelope.GetOrganization.OrganizationID,
		respEnvelope.GetOrganization.DisplayName)

	if respEnvelope.GetAccountDetails != nil {
		details = fmt.Sprintf("Email:             %s\n%s",
			respEnvelope.GetAccountDetails.EmailAddress,
			details)
	}

	// Add deployment access status
	if deployAccess != nil && deployAccess.HasAccess {
		details = fmt.Sprintf("%s\nDeploy Access:     Enabled", details)
	} else {
		details = fmt.Sprintf("%s\nDeploy Access:     Not enabled", details)
	}

	ui.Box(details)
	ui.Line()

	return nil
}
